package proxy

import (
	"bytes"
	"context"
	"cql-proxy/proxycore"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
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
// * Handle schema version and schema events. Need to pause for schema changes.
// * Handle endpoint factory refresh during total outage

const (
	maxPending = 1024
)

type Config struct {
	Version         primitive.ProtocolVersion
	Auth            proxycore.Authenticator
	Factory         proxycore.EndpointFactory
	ReconnectPolicy proxycore.ReconnectPolicy
	NumConns        int
}

type Proxy struct {
	ctx      context.Context
	config   Config
	listener net.Listener
	cluster  *proxycore.Cluster
	sessions sync.Map
	mu       *sync.Mutex
	logger   *zap.Logger
	wp       *workerPool
	localRow map[string]message.Column
}

func NewProxy(ctx context.Context, config Config) *Proxy {
	return &Proxy{
		ctx:      ctx,
		config:   config,
		sessions: sync.Map{},
		mu:       &sync.Mutex{},
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
	p.logger, err = zap.NewProduction()
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

	p.buildLocalRow()

	sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
		ReconnectPolicy: p.config.ReconnectPolicy,
		NumConns:        p.config.NumConns,
		Version:         p.cluster.NegotiatedVersion,
		Auth:            p.config.Auth,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	p.sessions.Store("", newSession(sess)) // No keyspace

	p.listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}

	p.wp = &workerPool{
		WorkerFunc:      serveRequest,
		MaxWorkersCount: 2, // TODO: Max count?
		Logger:          p.logger,
	}

	p.wp.Start()

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

func (p *Proxy) maybeCreateSession(keyspace string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.sessions.Load(keyspace); !ok {
		sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
			ReconnectPolicy: p.config.ReconnectPolicy,
			NumConns:        p.config.NumConns,
			Version:         p.cluster.NegotiatedVersion,
			Auth:            p.config.Auth,
			Keyspace:        keyspace,
		})
		if err != nil {
			return nil
		}
		p.sessions.Store(keyspace, sess)
	}
	return nil
}

var (
	schemaVersion, _ = primitive.ParseUuid("4f2b29e6-59b5-4e2d-8fd6-01e32e67f0d7")
	hostId, _        = primitive.ParseUuid("19e26944-ffb1-40a9-a184-a9b065e5e06b")
)

func (p *Proxy) buildLocalRow() {
	p.localRow = map[string]message.Column{
		"key":                     p.encodeTypeFatal(datatype.Varchar, "local"),
		"data_center":             p.encodeTypeFatal(datatype.Varchar, "dc1"),
		"rack":                    p.encodeTypeFatal(datatype.Varchar, "rack1"),
		"tokens":                  p.encodeTypeFatal(datatype.NewListType(datatype.Varchar), []string{"0"}),
		"release_version":         p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.ReleaseVersion),
		"partitioner":             p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.Partitioner),
		"cluster_name":            p.encodeTypeFatal(datatype.Varchar, "cql-proxy"),
		"cql_version":             p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.CQLVersion),
		"schema_version":          p.encodeTypeFatal(datatype.Uuid, schemaVersion), // TODO: Make this match the downstream cluster(s)
		"native_protocol_version": p.encodeTypeFatal(datatype.Varchar, p.cluster.NegotiatedVersion.String()),
		"host_id":                 p.encodeTypeFatal(datatype.Uuid, hostId),
	}
}

func (p *Proxy) encodeTypeFatal(dt datatype.DataType, val interface{}) []byte {
	bytes, err := proxycore.EncodeType(dt, p.cluster.NegotiatedVersion, val)
	if err != nil {
		p.logger.Fatal("unable to encode type", zap.Error(err))
	}
	return bytes
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
		pending: make(chan proxycore.Request, maxPending),
	}
}

func (c *client) Receive(reader io.Reader) error {
	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		return err
	}

	body, err := codec.DecodeBody(raw.Header, bytes.NewReader(raw.Body))
	if err != nil {
		c.proxy.logger.Error("unable to decode body", zap.Error(err))
		return err
	}

	switch msg := body.Message.(type) {
	case *message.Options:
		c.send(raw.Header, &message.Supported{Options: map[string][]string{"CQL_VERSION": {"3.0.0"}, "COMPRESSION": {}}})
	case *message.Startup:
		c.send(raw.Header, &message.Ready{})
	case *message.Register:
		c.send(raw.Header, &message.Ready{}) // TODO: Handle schema events
	case *message.Prepare:
		c.handlePrepare(raw.Header, msg)
	case *partialExecute:
		c.handleExecute(raw.Header, msg)
	case *partialQuery:
		c.handleQuery(raw.Header, msg)
	default:
		c.send(raw.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}

	//var r proxycore.Request

	//if s, ok := c.proxy.sessions.Load(c.keyspace); ok {
	//	s := s.(session)
	//	select {
	//	case <-s.session.IsConnected(): // TODO: Is this fast?
	//	default:
	//		select {
	//		case s.pending <- r:
	//		default:
	//			// TODO: Send back overwhelmed
	//		}
	//	}
	//}

	return nil
}

func (c *client) handlePrepare(hdr *frame.Header, msg *message.Prepare) {
	handled, idempotent, stmt := parse(c.keyspace, msg.Query)

	if handled {
		switch s := stmt.(type) {
		case *selectStatement:
			_ = s
		case *useStatement:
			_ = s
		case *errorSelectStatement:
			_ = s
		default:
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempt to intercept an unhandled query"})
		}
	} else {
		_ = idempotent
	}
}

func (c *client) handleExecute(hdr *frame.Header, msg *partialExecute) {
}

func (c *client) handleQuery(hdr *frame.Header, msg *partialQuery) {
	handled, idempotent, stmt := parse(c.keyspace, msg.query)

	c.proxy.logger.Info("handling query", zap.String("query", msg.query))

	if handled {
		switch s := stmt.(type) {
		case *selectStatement:
			if s.table == "local" {
				c.handleSystemQuery(hdr, s, c.proxy.localRow, systemLocalColumns, 1)
			} else if s.table == "peers" {
				c.handleSystemQuery(hdr, s, nil, systemPeersColumns, 0)
			} else {
				c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
			}
		case *useStatement:
			// TODO: Fix and put on worker pool
			go func() {
				c.proxy.maybeCreateSession(s.keyspace)
			}()
			c.keyspace = s.keyspace
			c.send(hdr, &message.VoidResult{})
		case *errorSelectStatement:
			c.send(hdr, &message.Invalid{ErrorMessage: s.err.Error()})
		default:
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
		}
	} else {
		_ = idempotent
		c.send(hdr, &message.VoidResult{})
	}
}

func (c *client) columnValue(values map[string]message.Column, name string) message.Column {
	var val message.Column
	var ok bool
	if val, ok = values[name]; !ok {
		if name == "rpc_address" {
			val, _ = proxycore.EncodeType(datatype.Inet, c.proxy.cluster.NegotiatedVersion, c.conn.LocalAddr())
		}
	}
	return val
}

func (c *client) handleSystemQuery(hdr *frame.Header, stmt *selectStatement, values map[string]message.Column,
	systemColumns []*message.ColumnMetadata, count int) {
	var row message.Row
	var columns []*message.ColumnMetadata
	if _, ok := stmt.selectors[0].(starSelector); ok {
		for _, column := range systemColumns {
			row = append(row, c.columnValue(values, column.Name))
		}
		columns = systemColumns
	} else {
		for _, selector := range stmt.selectors {
			val, column, err := c.handleSelector(selector, values, systemColumns, count, stmt.table)
			if err != nil {
				c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
				return
			}
			row = append(row, val)
			columns = append(columns, column)
		}
	}
	c.send(hdr, &message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: int32(len(columns)),
			Columns:     columns,
		},
		Data: []message.Row{row},
	})
}

func (c *client) handleSelector(selector interface{}, values map[string]message.Column,
	columns []*message.ColumnMetadata, count int, table string) (val message.Column, column *message.ColumnMetadata, err error) {
	switch s := selector.(type) {
	case *countStarSelector:
		val, _ = proxycore.EncodeType(datatype.Int, c.proxy.cluster.NegotiatedVersion, count)
		return val, &message.ColumnMetadata{
			Keyspace: "system",
			Table:    table,
			Name:     s.name,
			Type:     datatype.Int,
		}, nil
	case *idSelector:
		if column = findColumnMetadata(columns, s.name); column != nil {
			return c.columnValue(c.proxy.localRow, column.Name), column, nil
		} else {
			return nil, nil, fmt.Errorf("invalid column %s", s.name)
		}
	case *aliasSelector:
		val, column, err = c.handleSelector(s, values, columns, count, table)
		if err != nil {
			return nil, nil, err
		}
		alias := *column
		alias.Name = s.alias
		return val, &alias, nil
	default:
		return nil, nil, errors.New("unhandled selector type")
	}
}

func (c *client) send(hdr *frame.Header, msg message.Message) {
	c.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *client) Closing(err error) {
}
