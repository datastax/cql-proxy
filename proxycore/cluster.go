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
	"fmt"
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
	typ   ClusterEventType
	host  *Host
	hosts []*Host
}

type ClusterListener interface {
	OnEvent(event *ClusterEvent)
}

type ClusterConfig struct {
	Version         primitive.ProtocolVersion
	Auth            Authenticator
	Resolver        EndpointResolver
	ReconnectPolicy ReconnectPolicy
}

type ClusterInfo struct {
	Partitioner    string
	ReleaseVersion string
	CQLVersion     string
}

type Cluster struct {
	ctx              context.Context
	config           ClusterConfig
	controlConn      *ClientConn
	hosts            []*Host
	currentHostIndex int
	listeners        []ClusterListener
	addListener      chan ClusterListener
	events           chan *frame.Frame
	// the following are immutable after start up
	NegotiatedVersion primitive.ProtocolVersion
	Info              ClusterInfo
}

func ConnectCluster(ctx context.Context, config ClusterConfig) (*Cluster, error) {
	cluster := &Cluster{
		ctx:              ctx,
		config:           config,
		controlConn:      nil,
		hosts:            nil,
		currentHostIndex: 0,
		events:           make(chan *frame.Frame),
		addListener:      make(chan ClusterListener),
		listeners:        make([]ClusterListener, 0),
	}

	endpoints, err := config.Resolver.Resolve()
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints resolved")
	}

	for _, endpoint := range endpoints {
		err = cluster.connect(ctx, endpoint, true)
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

func (c *Cluster) OnEvent(frame *frame.Frame) {
	c.events <- frame
}

func (c *Cluster) connect(ctx context.Context, endpoint Endpoint, initial bool) error {
	conn, err := ConnectClientWithEvents(ctx, endpoint, c)
	if err != nil {
		return err
	}

	var version primitive.ProtocolVersion
	if initial {
		version = c.config.Version
	} else {
		version = c.NegotiatedVersion
	}

	negotiated, err := conn.Handshake(ctx, version, c.config.Auth)
	if err != nil {
		return err
	}
	if !initial && negotiated != version {
		return fmt.Errorf("unable to use required protocol version %v, got %v", version, negotiated)
	}

	hosts, info, err := c.queryHosts(ctx, conn, negotiated)
	if err != nil {
		return err
	}

	c.controlConn = conn
	if initial {
		c.NegotiatedVersion = negotiated
		c.Info = info
		c.hosts = hosts
	} else {
		c.mergeHosts(hosts)
	}

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
				typ:   ClusterEventAdded,
				host:  host,
				hosts: nil,
			})
		}
	}

	for _, host := range existing {
		c.sendEvent(&ClusterEvent{
			typ:   ClusterEventRemoved,
			host:  host,
			hosts: nil,
		})
	}

	c.hosts = hosts
}

func (c *Cluster) sendEvent(event *ClusterEvent) {
	for _, listener := range c.listeners {
		listener.OnEvent(event)
	}
}

func (c *Cluster) queryHosts(ctx context.Context, conn *ClientConn, version primitive.ProtocolVersion) (hosts []*Host, info ClusterInfo, err error) {
	var rs *ResultSet
	var val interface{}

	rs, err = conn.Query(ctx, version, &message.Query{
		Query: "SELECT * FROM system.local",
		Options: &message.QueryOptions{
			Consistency: primitive.ConsistencyLevelOne,
		},
	})
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	if rs.RowCount() == 0 {
		return nil, ClusterInfo{}, errors.New("empty result set returned for system.local")
	}
	hosts = c.addHosts(hosts, rs)
	row := rs.Row(0)

	val, err = row.ByName("partitioner")
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	partitioner := val.(string)

	val, err = row.ByName("release_version")
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	releaseVersion := val.(string)

	val, err = row.ByName("cql_version")
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	cqlVersion := val.(string)

	rs, err = conn.Query(ctx, version, &message.Query{
		Query: "SELECT * FROM system.peers",
		Options: &message.QueryOptions{
			Consistency: primitive.ConsistencyLevelOne,
		},
	})
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	hosts = c.addHosts(hosts, rs)

	return hosts, ClusterInfo{Partitioner: partitioner, ReleaseVersion: releaseVersion, CQLVersion: cqlVersion}, nil
}

func (c *Cluster) addHosts(hosts []*Host, rs *ResultSet) []*Host {
	for i := 0; i < rs.RowCount(); i++ {
		row := rs.Row(i)
		if endpoint, err := c.config.Resolver.Create(row); err == nil {
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
	err := c.connect(ctx, host.Endpoint(), false)
	if err != nil {
		log.Printf("error reconnecting to host %v: %v", host, err)
	}
}

func (c *Cluster) refreshHosts() {
	ctx, cancel := context.WithTimeout(context.Background(), RefreshTimeout)
	defer cancel()
	hosts, _, err := c.queryHosts(ctx, c.controlConn, c.NegotiatedVersion)
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
					typ:   ClusterEventBootstrap,
					host:  nil,
					hosts: c.hosts,
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
