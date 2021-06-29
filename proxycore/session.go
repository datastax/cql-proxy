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
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
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
	Logger          *zap.Logger
}

type Session struct {
	ctx       context.Context
	config    SessionConfig
	logger    *zap.Logger
	pools     sync.Map
	connected chan struct{}
	failed    chan error
}

func ConnectSession(ctx context.Context, cluster *Cluster, config SessionConfig) (*Session, error) {
	session := &Session{
		ctx:       ctx,
		config:    config,
		logger:    GetOrCreateNopLogger(config.Logger),
		pools:     sync.Map{},
		connected: make(chan struct{}),
		failed:    make(chan error, 1),
	}

	err := cluster.Listen(session)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-session.connected:
		return session, nil
	case err = <-session.failed:
		return nil, err
	}
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

func (s *Session) OnEvent(event interface{}) {
	switch evt := event.(type) {
	case *BootstrapEvent:
		go func() {
			var wg sync.WaitGroup

			count := len(evt.Hosts)
			wg.Add(count)

			for _, host := range evt.Hosts {
				go func(host *Host) {
					pool, err := connectPool(s.ctx, connPoolConfig{
						Endpoint:      host.Endpoint(),
						SessionConfig: s.config,
					})
					if err != nil {
						select {
						case s.failed <- err:
						default:
						}
					}
					s.pools.Store(host.Endpoint().Key(), pool)
					wg.Done()
				}(host)
			}

			wg.Wait()

			close(s.connected)
			close(s.failed)
		}()
	case *AddEvent:
		// There's no compute if absent for sync.Map, figure a better way to do this if the pool already exists.
		if pool, loaded := s.pools.LoadOrStore(evt.Host.Key(), connectPoolNoFail(s.ctx, connPoolConfig{
			Endpoint:      evt.Host.Endpoint(),
			SessionConfig: s.config,
		})); loaded {
			p := pool.(connPool)
			p.cancel()
		}
	case *RemoveEvent:
		if pool, ok := s.pools.LoadAndDelete(evt.Host.Key()); ok {
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
	config    connPoolConfig
	logger    *zap.Logger
	cancel    context.CancelFunc
	remaining int32
	conns     []*ClientConn
	connsMu   *sync.RWMutex
}

func connectPool(ctx context.Context, config connPoolConfig) (*connPool, error) {
	ctx, cancel := context.WithCancel(ctx)

	pool := &connPool{
		ctx:       ctx,
		config:    config,
		logger:    GetOrCreateNopLogger(config.Logger),
		cancel:    cancel,
		remaining: int32(config.NumConns),
		conns:     make([]*ClientConn, config.NumConns),
		connsMu:   &sync.RWMutex{},
	}

	errs := make([]error, config.NumConns)
	wg := sync.WaitGroup{}
	wg.Add(config.NumConns)

	for i := 0; i < config.NumConns; i++ {
		go func(idx int) {
			pool.conns[idx], errs[idx] = pool.connect()
			wg.Done()
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		pool.logger.Error("unable to connect pool", zap.Stringer("host", config.Endpoint), zap.Error(err))
		if err != nil && isCriticalErr(err) {
			return nil, err
		}
	}

	for i := 0; i < config.NumConns; i++ {
		go pool.stayConnected(i)
	}

	return pool, nil
}

func connectPoolNoFail(ctx context.Context, config connPoolConfig) *connPool {
	ctx, cancel := context.WithCancel(ctx)

	pool := &connPool{
		ctx:       ctx,
		config:    config,
		logger:    GetOrCreateNopLogger(config.Logger),
		cancel:    cancel,
		remaining: int32(config.NumConns),
		conns:     make([]*ClientConn, config.NumConns),
		connsMu:   &sync.RWMutex{},
	}

	for i := 0; i < config.NumConns; i++ {
		go pool.stayConnected(i)
	}

	return pool
}

func (p *connPool) leastBusyConn() *ClientConn {
	p.connsMu.RLock()
	defer p.connsMu.RUnlock()
	count := len(p.conns)
	if count == 0 {
		return nil
	} else if count == 1 {
		return p.conns[0]
	} else {
		idx := -1
		min := int32(math.MaxInt32)
		for i, conn := range p.conns {
			if conn != nil {
				inflight := conn.Inflight()
				if inflight < min {
					idx = i
					min = inflight
				}
			}
		}
		return p.conns[idx]
	}
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
		p.logger.Error("protocol version not support", zap.Stringer("wanted", p.config.Version), zap.Stringer("got", version))
		return nil, ProtocolNotSupported
	}
	if len(p.config.Keyspace) != 0 {
		err = conn.SetKeyspace(ctx, p.config.Version, p.config.Keyspace)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func (p *connPool) stayConnected(idx int) {
	conn := p.conns[idx]

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
					if err != nil {
						p.logger.Error("pool failed to connect", zap.Stringer("host", p.config.Endpoint), zap.Error(err))
					} else {
						p.connsMu.Lock()
						conn, p.conns[idx] = c, c
						p.connsMu.Unlock()
						reconnectPolicy.Reset()
						pendingConnect = false
					}
				}
			}
		} else {
			select {
			case <-p.ctx.Done():
				done = true
				_ = conn.Close()
			case <-conn.IsClosed():
				p.connsMu.Lock()
				conn, p.conns[idx] = nil, nil
				p.connsMu.Unlock()
			}
		}
	}
}
