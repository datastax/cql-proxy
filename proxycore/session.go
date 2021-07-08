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
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"log"
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
	ConnectTimeout  time.Duration
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
		log.Printf("#################### failed %v\n", err)
		return nil, err
	}
}

func (s *Session) Send(host *Host, request Request) error {
	conn := s.leastBusyConn(host)
	if conn == nil {
		return NoConnForHost
	}
	return conn.Send(request)
}

func (s *Session) leastBusyConn(host *Host) *ClientConn {
	if p, ok := s.pools.Load(host.Endpoint().Key()); ok {
		pool := p.(*connPool)
		return pool.leastBusyConn()
	}
	return nil
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
						log.Printf("################### Adding error %v\n", err)
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
			p := pool.(*connPool)
			p.cancel()
		}
	case *RemoveEvent:
		if pool, ok := s.pools.LoadAndDelete(evt.Host.Key()); ok {
			p := pool.(*connPool)
			p.cancel()
		}
	}
}
