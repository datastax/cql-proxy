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
	"math"
	"sync"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

type connPoolConfig struct {
	Endpoint
	SessionConfig
}

type connPool struct {
	ctx           context.Context
	config        connPoolConfig
	logger        *zap.Logger
	preparedCache PreparedCache
	cancel        context.CancelFunc
	conns         []*ClientConn
	connsMu       *sync.RWMutex
}

// connectPool establishes a pool of connections to a given endpoint within a downstream cluster. These connection pools will
// be used to proxy requests from the client to the cluster.
func connectPool(ctx context.Context, config connPoolConfig) (*connPool, error) {
	ctx, cancel := context.WithCancel(ctx)

	pool := &connPool{
		ctx:           ctx,
		config:        config,
		logger:        GetOrCreateNopLogger(config.Logger),
		preparedCache: config.PreparedCache,
		cancel:        cancel,
		conns:         make([]*ClientConn, config.NumConns),
		connsMu:       &sync.RWMutex{},
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
		if err != nil {
			pool.logger.Error("unable to connect pool", zap.Stringer("host", config.Endpoint), zap.Error(err))
			if isCriticalErr(err) {
				return nil, err
			}
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
		ctx:     ctx,
		config:  config,
		logger:  GetOrCreateNopLogger(config.Logger),
		cancel:  cancel,
		conns:   make([]*ClientConn, config.NumConns),
		connsMu: &sync.RWMutex{},
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
		idx := 0
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

func (p *connPool) connect() (conn *ClientConn, err error) {
	timeout := getOrUseDefault(p.config.ConnectTimeout, DefaultConnectTimeout)
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	conn, err = ConnectClient(ctx, p.config.Endpoint, ClientConnConfig{
		PreparedCache: p.preparedCache,
		Logger:        p.logger})
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()

	var version primitive.ProtocolVersion
	version, err = conn.Handshake(ctx, p.config.Version, p.config.Auth)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("handshake took longer than %s to complete", timeout)
		}
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

	go conn.Heartbeats(timeout, p.config.Version, p.config.HeartBeatInterval, p.config.IdleTimeout, p.logger)
	return conn, nil
}

// stayConnected will attempt to reestablish a disconnected (`connection == nil`) connection within the pool. Reconnect attempts
// will be made at intervals defined by the ReconnectPolicy.
func (p *connPool) stayConnected(idx int) {
	conn := p.conns[idx]

	connectTimer := time.NewTimer(0)
	reconnectPolicy := p.config.ReconnectPolicy.Clone()
	pendingConnect := true

	done := false
	for !done {
		if conn == nil {
			if !pendingConnect {
				delay := reconnectPolicy.NextDelay()
				p.logger.Info("pool connection attempting to reconnect after delay",
					zap.Stringer("host", p.config.Endpoint), zap.Duration("delay", delay))
				connectTimer = time.NewTimer(reconnectPolicy.NextDelay())
				pendingConnect = true
			} else {
				select {
				case <-p.ctx.Done():
					done = true
				case <-connectTimer.C:
					c, err := p.connect()
					if err != nil {
						p.logger.Error("pool connection failed to connect",
							zap.Stringer("host", p.config.Endpoint), zap.Error(err))
					} else {
						p.connsMu.Lock()
						conn, p.conns[idx] = c, c
						p.connsMu.Unlock()
						reconnectPolicy.Reset()
					}
					pendingConnect = false
				}
			}
		} else {
			select {
			case <-p.ctx.Done():
				done = true
				_ = conn.Close()
			case <-conn.IsClosed():
				p.logger.Info("pool connection closed", zap.Stringer("host", p.config.Endpoint))
				p.connsMu.Lock()
				conn, p.conns[idx] = nil, nil
				p.connsMu.Unlock()
				pendingConnect = false
			}
		}
	}
}
