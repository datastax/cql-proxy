// Copyright (c) DataStax, Inc.
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
	"sort"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

const (
	DefaultRefreshWindow  = 10 * time.Second
	DefaultConnectTimeout = 10 * time.Second
	DefaultRefreshTimeout = 5 * time.Second
)

type AddEvent struct {
	Host *Host
}

type RemoveEvent struct {
	Host *Host
}

type BootstrapEvent struct {
	Hosts []*Host
}

type SchemaChangeEvent struct {
	Message *message.SchemaChangeEvent
}

type ReconnectEvent struct {
	Endpoint
}

type ClusterListener interface {
	OnEvent(event interface{})
}

type ClusterListenerFunc func(event interface{})

func (f ClusterListenerFunc) OnEvent(event interface{}) {
	f(event)
}

type ClusterConfig struct {
	Version           primitive.ProtocolVersion
	Auth              Authenticator
	Resolver          EndpointResolver
	ReconnectPolicy   ReconnectPolicy
	RefreshWindow     time.Duration
	ConnectTimeout    time.Duration
	RefreshTimeout    time.Duration
	Logger            *zap.Logger
	HeartBeatInterval time.Duration
	IdleTimeout       time.Duration
}

type ClusterInfo struct {
	Partitioner    string
	ReleaseVersion string
	CQLVersion     string
}

// Cluster defines a downstream cluster that is being proxied to.
type Cluster struct {
	ctx              context.Context
	config           ClusterConfig
	logger           *zap.Logger
	controlConn      *ClientConn
	currentEndpoint  Endpoint
	hosts            []*Host
	currentHostIndex int
	listeners        []ClusterListener
	addListener      chan ClusterListener
	events           chan *frame.Frame
	// the following are immutable after start up
	NegotiatedVersion primitive.ProtocolVersion
	Info              ClusterInfo
}

// ConnectCluster establishes control connections to each of the endpoints within a downstream cluster that is being proxied to.
func ConnectCluster(ctx context.Context, config ClusterConfig) (*Cluster, error) {
	c := &Cluster{
		ctx:              ctx,
		config:           config,
		logger:           GetOrCreateNopLogger(config.Logger),
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
		err = c.connect(ctx, endpoint, true)
		if err == nil {
			c.logger.Debug("control connection connected", zap.Stringer("endpoint", c.currentEndpoint))
			break
		}
	}

	if err != nil {
		return nil, err
	}

	go c.stayConnected()

	return c, nil
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

func (c *Cluster) connect(ctx context.Context, endpoint Endpoint, initial bool) (err error) {
	timeout := getOrUseDefault(c.config.ConnectTimeout, DefaultConnectTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := ConnectClient(ctx, endpoint, ClientConnConfig{Handler: c, Logger: c.logger})
	if err != nil {
		return err
	}

	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()

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

	c.currentEndpoint = endpoint
	c.controlConn = conn
	if initial {
		c.NegotiatedVersion = negotiated
		c.Info = info
	}

	go conn.Heartbeats(timeout, version, c.config.HeartBeatInterval, c.config.IdleTimeout, c.logger)

	return c.mergeHosts(hosts)
}

func (c *Cluster) mergeHosts(hosts []*Host) error {
	existing := make(map[string]*Host)

	for _, host := range c.hosts {
		existing[host.Endpoint().Key()] = host
	}

	c.currentHostIndex = -1
	for i, host := range hosts {
		if host.Key() == c.currentEndpoint.Key() {
			c.currentHostIndex = i
			break
		}
	}
	if c.currentHostIndex < 0 {
		return fmt.Errorf("host %s not found in system tables", c.currentEndpoint)
	}

	for _, host := range hosts {
		key := host.Key()
		if _, ok := existing[key]; ok {
			delete(existing, key)
		} else {
			c.sendEvent(&AddEvent{host})
		}
	}

	for _, host := range existing {
		c.sendEvent(&RemoveEvent{host})
	}

	c.hosts = hosts

	return nil
}

func (c *Cluster) sendEvent(event interface{}) {
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

	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Key() < hosts[j].Key()
	})

	return hosts, ClusterInfo{Partitioner: partitioner, ReleaseVersion: releaseVersion, CQLVersion: cqlVersion}, nil
}

func (c *Cluster) addHosts(hosts []*Host, rs *ResultSet) []*Host {
	for i := 0; i < rs.RowCount(); i++ {
		row := rs.Row(i)
		if endpoint, err := c.config.Resolver.NewEndpoint(row); err == nil {
			if host, err := NewHostFromRow(endpoint, row); err == nil {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
}

func (c *Cluster) reconnect() bool {
	c.currentHostIndex = (c.currentHostIndex + 1) % len(c.hosts)
	host := c.hosts[c.currentHostIndex]
	err := c.connect(c.ctx, host.Endpoint(), false)
	if err != nil {
		c.logger.Error("error reconnecting to host", zap.Stringer("host", host), zap.Error(err))
		return false
	} else {
		c.logger.Debug("control connection connected", zap.Stringer("endpoint", c.currentEndpoint))
		c.sendEvent(&ReconnectEvent{c.currentEndpoint})
		return true
	}
}

func (c *Cluster) refreshHosts() {
	timeout := getOrUseDefault(c.config.RefreshTimeout, DefaultRefreshTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	hosts, _, err := c.queryHosts(ctx, c.controlConn, c.NegotiatedVersion)
	if err == nil {
		err = c.mergeHosts(hosts)
	}
	if err != nil {
		c.logger.Error("unable to refresh hosts", zap.Error(err))
		_ = c.controlConn.Close()
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
				delay := reconnectPolicy.NextDelay()
				c.logger.Debug("control connection attempting to reconnect after delay", zap.Duration("delay", delay))
				connectTimer = time.NewTimer(delay)
				pendingConnect = true
			} else {
				select {
				case <-c.ctx.Done():
					done = true
				case <-connectTimer.C:
					if c.reconnect() {
						reconnectPolicy.Reset()
					}
					pendingConnect = false
				}
			}
		} else {
			select {
			case <-c.ctx.Done():
				done = true
				_ = c.controlConn.Close()
			case <-c.controlConn.IsClosed():
				c.logger.Warn("control connection closed", zap.Stringer("endpoint", c.currentEndpoint), zap.Error(c.controlConn.Err()))
				c.controlConn = nil
			case newListener := <-c.addListener:
				for _, listener := range c.listeners {
					if newListener == listener {
						continue
					}
				}
				newListener.OnEvent(&BootstrapEvent{c.hosts})
				c.listeners = append(c.listeners, newListener)
			case <-refreshTimer.C:
				c.refreshHosts()
				pendingRefresh = false
			case event := <-c.events:
				window := getOrUseDefault(c.config.RefreshWindow, DefaultRefreshWindow)
				switch msg := event.Body.Message.(type) {
				case *message.TopologyChangeEvent:
					if !pendingRefresh {
						refreshTimer = time.NewTimer(window)
						pendingRefresh = true
					}
				case *message.StatusChangeEvent:
					if !pendingRefresh && msg.ChangeType == primitive.StatusChangeTypeUp {
						refreshTimer = time.NewTimer(window)
						pendingRefresh = true
					}
				case *message.SchemaChangeEvent:
					for _, listener := range c.listeners {
						listener.OnEvent(&SchemaChangeEvent{Message: msg})
					}
				}
			}
		}
	}
}

func getOrUseDefault(time time.Duration, def time.Duration) time.Duration {
	if time == 0 {
		return def
	}
	return time
}
