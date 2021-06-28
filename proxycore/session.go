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
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"math"
	"sync"
	"time"
)

var (
	NoConnForHost = errors.New("no connection available for host")
)

type SessionConfig struct {
	ReconnectPolicy ReconnectPolicy
	NumConns        int
	Keyspace        string
	Version         primitive.ProtocolVersion
	Auth            Authenticator
}

type Session struct {
	ctx       context.Context
	connected chan struct{}
	config    SessionConfig
	pools     sync.Map
}

func ConnectSession(ctx context.Context, cluster *Cluster, config SessionConfig) (*Session, error) {
	session := &Session{
		ctx:       ctx,
		connected: make(chan struct{}),
		config:    config,
		pools:     sync.Map{},
	}

	err := cluster.Listen(session)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Session) Send(host *Host, request Request) error {
	var conn *ClientConn
	if p, ok := s.pools.Load(host.Endpoint().Key()); ok {
		pool := p.(*connPool)
		conn = pool.leastBusyConn()
	}
	if conn == nil {
		return NoConnForHost
	}
	return conn.Send(request)
}

func (s *Session) IsConnected() chan struct{} {
	return s.connected
}

func (s *Session) OnEvent(event *ClusterEvent) {
	switch event.typ {
	case ClusterEventBootstrap:
		go func() {
			pools := make([]*connPool, 0, len(event.hosts))
			for _, host := range event.hosts {
				pool := connectPool(s.ctx, connPoolConfig{
					Endpoint:      host.Endpoint(),
					SessionConfig: s.config,
				})
				pools = append(pools, pool)
				s.pools.Store(host.Endpoint().Key(), pool)
			}

			for _, pool := range pools {
				select {
				case <-pool.isConnected():
				case <-s.ctx.Done():
				}
			}
			close(s.connected)
		}()
	case ClusterEventAdded:
		// There's no compute if absent for sync.Map, figure a better way to do this if the pool already exists.
		if pool, loaded := s.pools.LoadOrStore(event.host.Endpoint().Key(), connectPool(s.ctx, connPoolConfig{
			Endpoint:      event.host.Endpoint(),
			SessionConfig: s.config,
		})); loaded {
			p := pool.(connPool)
			p.cancel()
		}
	case ClusterEventRemoved:
		if pool, ok := s.pools.LoadAndDelete(event.host.Endpoint().Key()); ok {
			p := pool.(connPool)
			p.cancel()
		}
	}
}

type connPoolConfig struct {
	Endpoint
	SessionConfig
}

type connPool struct {
	ctx       context.Context
	cancel    context.CancelFunc
	connected chan struct{}
	remaining int32
	config    connPoolConfig
	conns     []*ClientConn
	mu        *sync.RWMutex
}

func connectPool(ctx context.Context, config connPoolConfig) *connPool {
	ctx, cancel := context.WithCancel(ctx)

	pool := &connPool{
		ctx:       ctx,
		cancel:    cancel,
		connected: make(chan struct{}),
		remaining: int32(config.NumConns),
		config:    config,
		conns:     make([]*ClientConn, config.NumConns),
		mu:        &sync.RWMutex{},
	}

	for i := 0; i < config.NumConns; i++ {
		go pool.stayConnected(i)
	}

	return pool
}

func (p *connPool) leastBusyConn() *ClientConn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := len(p.conns)
	if count == 0 {
		return nil
	} else if count == 1 {
		return p.conns[0]
	} else {
		index := -1
		min := int32(math.MaxInt32)
		for i, conn := range p.conns {
			if conn != nil {
				inflight := conn.Inflight()
				if inflight < min {
					index = i
					min = inflight
				}
			}
		}
		return p.conns[index]
	}
}

func (p *connPool) isConnected() chan struct{} {
	return p.connected
}

func (p *connPool) maybeConnected() {
	p.mu.Lock()
	if p.remaining > 0 {
		p.remaining--
		if p.remaining == 0 {
			close(p.connected)
		}
	}
	p.mu.Unlock()
}

func (p *connPool) connect() (*ClientConn, error) {
	ctx, cancel := context.WithTimeout(p.ctx, ConnectTimeout)
	defer cancel()
	conn, err := ConnectClient(ctx, p.config.Endpoint)
	if err != nil {
		return nil, err
	}
	var version primitive.ProtocolVersion
	version, err = conn.Handshake(ctx, p.config.Version, p.config.Auth)
	if err != nil {
		return nil, err
	}
	if version != p.config.Version {
		return nil, fmt.Errorf("protocol version %v not support, got %v", p.config.Version, version)
	}
	if len(p.config.Keyspace) != 0 {
		_, err = conn.Query(ctx, p.config.Version, &message.Query{
			Query: fmt.Sprintf("USE %s", p.config.Keyspace),
		})
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func (p *connPool) stayConnected(index int) {
	var conn *ClientConn

	connectTimer := time.NewTimer(0)
	reconnectPolicy := p.config.ReconnectPolicy.Clone()
	pendingConnect := false

	done := false
	for !done {
		if conn == nil {
			if !pendingConnect {
				connectTimer = time.NewTimer(reconnectPolicy.NextDelay())
				pendingConnect = true
			} else {
				select {
				case <-p.ctx.Done():
					done = true
				case <-connectTimer.C:
					c, err := p.connect()
					if err == nil {
						p.mu.Lock()
						conn, p.conns[index] = c, c
						p.mu.Unlock()
						reconnectPolicy.Reset()
						pendingConnect = false
					} else {
						// TODO: Fatal errors: protocol version, keyspace, auth
					}
					p.maybeConnected()
				}
			}
		} else {
			select {
			case <-p.ctx.Done():
				done = true
				_ = conn.Close()
			case <-conn.IsClosed():
				p.mu.Lock()
				conn, p.conns[index] = nil, nil
				p.mu.Unlock()
			}
		}
	}
}
