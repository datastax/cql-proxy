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
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"math"
	"sync"
	"time"
)

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
	conn, err = ConnectClient(ctx, p.config.Endpoint)
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
				delay := reconnectPolicy.NextDelay()
				p.logger.Debug("pool connection attempting to reconnect after delay", zap.Duration("delay", delay))
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
				p.connsMu.Lock()
				conn, p.conns[idx] = nil, nil
				p.connsMu.Unlock()
			}
		}
	}
}
