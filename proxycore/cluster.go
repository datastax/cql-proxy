// Copyright 2020 DataStax
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxycore

import (
	"context"
	"errors"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"log"
	"time"
)

const (
	RefreshWindow  = 10 * time.Second
	ConnectTimeout = 10 * time.Second
	RefreshTimeout = 5 * time.Second
)

type ClusterEventType int

const (
	ClusterEventBootstrap = iota
	ClusterEventAdded
	ClusterEventRemoved
)

type ClusterEvent struct {
	eventType ClusterEventType
	host      *Host
	hosts     []*Host
}

type ClusterListener interface {
	OnEvent(event *ClusterEvent)
}

type ClusterConfig struct {
	Version         primitive.ProtocolVersion
	Auth            Authenticator
	Factory         EndpointFactory
	ReconnectPolicy ReconnectPolicy
}

type Cluster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	config           ClusterConfig
	controlConn      *ClusterConn
	hosts            []*Host
	currentHostIndex int
	listeners        []ClusterListener
	addListener      chan ClusterListener
	events           chan *frame.Frame
}

func ConnectToCluster(ctx context.Context, config ClusterConfig) (*Cluster, error) {
	if len(config.Factory.ContactPoints()) == 0 {
		return nil, errors.New("no endpoints resolved")
	}

	ctx, cancel := context.WithCancel(ctx)

	cluster := &Cluster{
		ctx:              ctx,
		cancel:           cancel,
		config:           config,
		controlConn:      nil,
		hosts:            nil,
		currentHostIndex: 0,
		events:           make(chan *frame.Frame),
		addListener:      make(chan ClusterListener),
		listeners:        make([]ClusterListener, 0),
	}

	var err error
	for _, endpoint := range config.Factory.ContactPoints() {
		err = cluster.connect(ctx, endpoint)
	}

	if err != nil {
		return nil, err
	}

	go cluster.stayConnected()

	return cluster, nil
}

func (c *Cluster) Listen(listener ClusterListener) error {
	select {
	case c.addListener <- listener:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *Cluster) Context() context.Context {
	return c.ctx
}

func (c *Cluster) Cancel() {
	c.cancel()
}

func (c *Cluster) OnEvent(frame *frame.Frame) {
	c.events <- frame
}

func (c *Cluster) connect(ctx context.Context, endpoint Endpoint) error {
	conn, err := ClusterConnectWithEvents(ctx, endpoint, c)
	if err != nil {
		return err
	}

	negotiated, err := conn.Handshake(ctx, c.config.Version, c.config.Auth)
	if err != nil {
		return err
	}

	hosts, err := c.queryHosts(ctx, conn)
	if err != nil {
		return err
	}

	c.controlConn = conn
	if c.hosts == nil {
		c.hosts = hosts
	} else {
		c.mergeHosts(hosts)
	}
	c.config.Version = negotiated

	return nil
}

func (c *Cluster) mergeHosts(hosts []*Host) {
	existing := make(map[string]*Host)

	for _, host := range c.hosts {
		existing[host.Endpoint().Key()] = host
	}

	currentHostKey := c.hosts[c.currentHostIndex].Endpoint().Key()

	for i, host := range hosts {
		key := host.Endpoint().Key()
		if key == currentHostKey {
			c.currentHostIndex = i
		}
		if _, ok := existing[key]; ok {
			delete(existing, key)
		} else {
			c.sendEvent(&ClusterEvent{
				eventType: ClusterEventAdded,
				host:      host,
				hosts:     nil,
			})
		}
	}

	for _, host := range existing {
		c.sendEvent(&ClusterEvent{
			eventType: ClusterEventRemoved,
			host:      host,
			hosts:     nil,
		})
	}

	c.hosts = hosts
}

func (c *Cluster) sendEvent(event *ClusterEvent) {
	for _, listener := range c.listeners {
		listener.OnEvent(event)
	}
}

func (c *Cluster) queryHosts(ctx context.Context, conn *ClusterConn) ([]*Host, error) {
	var rs *ResultSet
	var err error
	hosts := make([]*Host, 0)

	rs, err = conn.Query(ctx, c.config.Version, &message.Query{
		Query: "SELECT * FROM system.local",
		Options: &message.QueryOptions{
			Consistency: primitive.ConsistencyLevelOne,
		},
	})
	if err != nil {
		return nil, err
	}
	hosts = c.addHosts(hosts, rs)

	rs, err = conn.Query(ctx, c.config.Version, &message.Query{
		Query: "SELECT * FROM system.peers",
		Options: &message.QueryOptions{
			Consistency: primitive.ConsistencyLevelOne,
		},
	})
	if err != nil {
		return nil, err
	}
	hosts = c.addHosts(hosts, rs)

	return hosts, nil
}

func (c *Cluster) addHosts(hosts []*Host, rs *ResultSet) []*Host {
	for i := 0; i < rs.RowCount(); i++ {
		row := rs.Row(i)
		if endpoint, err := c.config.Factory.Create(row); err == nil {
			if host, err := NewHostFromRow(endpoint, row); err == nil {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
}

func (c *Cluster) reconnect() {
	c.currentHostIndex = (c.currentHostIndex + 1) % len(c.hosts)
	host := c.hosts[c.currentHostIndex]
	ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
	defer cancel()
	err := c.connect(ctx, host.Endpoint())
	if err != nil {
		log.Printf("error reconnecting to host %v: %v", host, err)
	}
}

func (c *Cluster) refreshHosts() {
	ctx, cancel := context.WithTimeout(context.Background(), RefreshTimeout)
	defer cancel()
	hosts, err := c.queryHosts(ctx, c.controlConn)
	if err != nil {
		log.Printf("unable to refresh hosts: %v", err)
		_ = c.controlConn.Close()
	} else {
		c.mergeHosts(hosts)
	}
}

func (c *Cluster) stayConnected() {
	refreshTimer := time.NewTimer(0)
	refreshTimer.Stop()
	pendingRefresh := false

	connectTimer := time.NewTimer(0)
	connectTimer.Stop()
	reconnectPolicy := c.config.ReconnectPolicy.Clone()
	pendingConnect := false

	done := false

	for !done {
		if c.controlConn == nil {
			if !pendingConnect {
				connectTimer = time.NewTimer(reconnectPolicy.NextDelay())
				pendingConnect = true
			} else {
				select {
				case <-c.ctx.Done():
					done = true
					_ = c.controlConn.Close()
				case <-connectTimer.C:
					c.reconnect()
					reconnectPolicy.Reset()
					pendingConnect = false
				}
			}
		} else {
			select {
			case <-c.ctx.Done():
				done = true
				_ = c.controlConn.Close()
			case <-c.controlConn.IsClosed():
				c.controlConn = nil
			case newListener := <-c.addListener:
				for _, listener := range c.listeners {
					if newListener == listener {
						continue
					}
				}
				newListener.OnEvent(&ClusterEvent{
					eventType: ClusterEventBootstrap,
					host:      nil,
					hosts:     c.hosts,
				})
				c.listeners = append(c.listeners, newListener)
			case <-refreshTimer.C:
				c.refreshHosts()
				pendingRefresh = false
			case event := <-c.events:
				switch msg := event.Body.Message.(type) {
				case *message.TopologyChangeEvent:
					if !pendingRefresh {
						refreshTimer = time.NewTimer(RefreshWindow)
						pendingRefresh = true
					}
				case *message.StatusChangeEvent:
					if !pendingRefresh && msg.ChangeType == primitive.StatusChangeTypeUp {
						refreshTimer = time.NewTimer(RefreshWindow)
						pendingRefresh = true
					}
				}
			}
		}
	}
}
