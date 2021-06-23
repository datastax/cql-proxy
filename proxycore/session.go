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
	cancel    context.CancelFunc
	connected chan struct{}
	config    SessionConfig
	pools     sync.Map
}

func SessionConnect(ctx context.Context, cluster *Cluster, config SessionConfig) (*Session, error) {
	ctx, cancel := context.WithCancel(ctx)

	session := &Session{
		ctx:       ctx,
		cancel:    cancel,
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
	var conn *ClusterConn
	if p, ok := s.pools.Load(host.Endpoint().Key()); ok {
		pool := p.(ConnPool)
		conn = pool.LeastBusyConn()
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
	switch event.eventType {
	case ClusterEventBootstrap:
		go func() {
			pools := make([]*ConnPool, 0, len(event.hosts))
			for _, host := range event.hosts {
				pool := PoolConnect(s.ctx, ConnPoolConfig{
					Endpoint:      host.Endpoint(),
					SessionConfig: s.config,
				})
				pools = append(pools, pool)
				s.pools.Store(host.Endpoint().Key(), pool)
			}

			for _, pool := range pools {
				select {
				case <-pool.IsConnected():
				case <-s.ctx.Done():
				}
			}
			close(s.connected)
		}()
	case ClusterEventAdded:
		// There's no compute if absent for sync.Map, figure a better way to do this if the pool already exists.
		if pool, loaded := s.pools.LoadOrStore(event.host.Endpoint().Key(), PoolConnect(s.ctx, ConnPoolConfig{
			Endpoint:      event.host.Endpoint(),
			SessionConfig: s.config,
		})); loaded {
			p := pool.(ConnPool)
			p.Cancel()
		}
	case ClusterEventRemoved:
		if pool, ok := s.pools.LoadAndDelete(event.host.Endpoint().Key()); ok {
			p := pool.(ConnPool)
			p.Cancel()
		}
	}
}

type ConnPoolConfig struct {
	Endpoint
	SessionConfig
}

type ConnPool struct {
	ctx       context.Context
	cancel    context.CancelFunc
	connected chan struct{}
	remaining int32
	config    ConnPoolConfig
	conns     []*ClusterConn
	mu        *sync.RWMutex
}

func PoolConnect(ctx context.Context, config ConnPoolConfig) *ConnPool {
	ctx, cancel := context.WithCancel(ctx)

	pool := &ConnPool{
		ctx:       ctx,
		cancel:    cancel,
		connected: make(chan struct{}),
		remaining: int32(config.NumConns),
		config:    config,
		conns:     make([]*ClusterConn, config.NumConns),
		mu:        &sync.RWMutex{},
	}

	for i := 0; i < config.NumConns; i++ {
		go pool.stayConnected(i)
	}

	return pool
}

func (p *ConnPool) LeastBusyConn() *ClusterConn {
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
		if index >= 0 {
			return p.conns[index]
		} else {
			return nil
		}
	}
}

func (p *ConnPool) Context() context.Context {
	return p.ctx
}

func (p *ConnPool) Cancel() {
	p.cancel()
}

func (p *ConnPool) IsConnected() chan struct{} {
	return p.connected
}

func (p *ConnPool) maybeConnected() {
	p.mu.Lock()
	if p.remaining > 0 {
		p.remaining--
		if p.remaining == 0 {
			close(p.connected)
		}
	}
	p.mu.Unlock()
}

func (p *ConnPool) connect() (*ClusterConn, error) {
	ctx, cancel := context.WithTimeout(p.ctx, ConnectTimeout)
	defer cancel()
	conn, err := ClusterConnect(ctx, p.config.Endpoint)
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

func (p *ConnPool) stayConnected(index int) {
	var conn *ClusterConn

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
						// TODO
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
