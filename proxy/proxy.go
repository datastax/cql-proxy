package proxy

import (
	"context"
	"cql-proxy/proxycore"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"io"
	"net"
	"sync"
)

// TODO:

// # Frame parsing
// * Learn: github.com/datastax/go-cassandra-native-protocol
// # Result set construction and parsing

// # Backend
// * Proxy-to-server CQL connection
//   - Read/Write requests
//   - Retry when down until removed (exponential backoff)
//   - Heartbeat
//   - Stream management
// * Control connection
//   - Query system.local/system.peers
//   - ADD/REMOVE/UP and schema events (channels)
//   - Contact point resolver
// * Sessions
//   - Pool connections and connection lifecycle
//   - Simple load-balancing (round-robin to start), concurrency!
//   - Keyspace state (USE <keyspace> problem, intercept and create new sessions)
// * Cloud
//   - Metadata service contact point resolver
//   - Endpoint type (with cluster DNS and SNI name, TLS config?)
//   - Make sure DNS round-robins A-records

// # Frontend
// * Client-to-proxy CQL connection, worker pool, httpfast
// * Fast CQL parser (limited recursive descent parser?)
//   - Intercept `system.local` and `system.peers` queries and USE <keyspace>
//   - Example: https://github.com/mpenick/cql-proxy/blob/main/src/parse.h
// * Pass through other query types, raw

// * Handle lazy USE keyspace
// * Retry connect pool on UP events?
// * Share connect pool DOWN events with Cluster (control connection)?
// * Handle mixed protocol versions e.g. client = V3, server = V4?

const (
	MaxPending = 1024
)

type Config struct {
	Version         primitive.ProtocolVersion
	Auth            proxycore.Authenticator
	Factory         proxycore.EndpointFactory
	ReconnectPolicy proxycore.ReconnectPolicy
	NumConns        int
	Codec           frame.RawCodec
}

type Proxy struct {
	ctx      context.Context
	config   Config
	listener net.Listener
	cluster  *proxycore.Cluster
	sessions sync.Map
	log      *zap.Logger
}

func NewProxy(ctx context.Context, config Config) *Proxy {
	return &Proxy{
		ctx:      ctx,
		config:   config,
		sessions: sync.Map{},
	}
}

func (p *Proxy) ListenAndServe(address string) error {
	err := p.Listen(address)
	if err != nil {
		return err
	}
	return p.Serve()
}

func (p *Proxy) Listen(address string) error {
	var err error
	p.log, err = zap.NewProduction()
	if err != nil {
		return fmt.Errorf("unable to create logger %w", err)
	}

	p.cluster, err = proxycore.ConnectCluster(p.ctx, proxycore.ClusterConfig{
		Version:         p.config.Version,
		Auth:            p.config.Auth,
		Factory:         p.config.Factory,
		ReconnectPolicy: p.config.ReconnectPolicy,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	s, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
		ReconnectPolicy: p.config.ReconnectPolicy,
		NumConns:        p.config.NumConns,
		Version:         p.config.Version, // TODO: Fix, this should use the negotiated version from Cluster
		Auth:            p.config.Auth,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	p.sessions.Store("", newSession(s)) // No keyspace

	p.listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}

	return nil
}

func (p *Proxy) Serve() error {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return err
		}
		p.handle(conn)
	}
}

func (p *Proxy) handle(conn net.Conn) {
	cl := &client{
		ctx:   p.ctx,
		proxy: p,
	}
	cl.conn = proxycore.NewConn(conn, cl)
	cl.conn.Start()
}

type client struct {
	ctx      context.Context
	proxy    *Proxy
	conn     *proxycore.Conn
	keyspace string
}

type session struct {
	session *proxycore.Session
	pending chan proxycore.Request
}

func newSession(s *proxycore.Session) *session {
	return &session{
		session: s,
		pending: make(chan proxycore.Request, MaxPending),
	}
}

func (c *client) Receive(reader io.Reader) error {
	decoded, err := c.proxy.config.Codec.DecodeRawFrame(reader)
	if err != nil {
		return err
	}

	_ = decoded

	var r proxycore.Request

	if s, ok := c.proxy.sessions.Load(c.keyspace); ok {
		s := s.(session)
		select {
		case <-s.session.IsConnected(): // TODO: Is this fast?
		default:
			select {
			case s.pending <- r:
			default:
				// TODO: Send back overwhelmed
			}
		}
	}

	return nil
}

func (c *client) Closing(err error) {
}
