package proxy

import (
	"bytes"
	"context"
	"cql-proxy/proxycore"
	"crypto/md5"
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
// * Handle endpoint resolver refresh during total outage
// * Handle critical errors when connecting a new session/connection pool
// * Automatic retries when a query is idempotent

const (
	maxPending = 1024
)

type Config struct {
	Version         primitive.ProtocolVersion
	Auth            proxycore.Authenticator
	Resolver        proxycore.EndpointResolver
	ReconnectPolicy proxycore.ReconnectPolicy
	NumConns        int
	Logger          *zap.Logger
}

type Proxy struct {
	ctx      context.Context
	config   Config
	logger   *zap.Logger
	listener net.Listener
	cluster  *proxycore.Cluster
	sessions sync.Map
	sessMu   *sync.Mutex
	wp       *workerPool
	lb       proxycore.LoadBalancer
	localRow map[string]message.Column
}

func NewProxy(ctx context.Context, config Config) *Proxy {
	return &Proxy{
		ctx:      ctx,
		config:   config,
		logger:   proxycore.GetOrCreateNopLogger(config.Logger),
		sessions: sync.Map{},
		sessMu:   &sync.Mutex{},
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

	p.cluster, err = proxycore.ConnectCluster(p.ctx, proxycore.ClusterConfig{
		Version:         p.config.Version,
		Auth:            p.config.Auth,
		Resolver:        p.config.Resolver,
		ReconnectPolicy: p.config.ReconnectPolicy,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	p.buildLocalRow()

	p.lb = proxycore.NewRoundRobinLoadBalancer()
	err = p.cluster.Listen(p.lb)
	if err != nil {
		return err
	}

	sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
		ReconnectPolicy: p.config.ReconnectPolicy,
		NumConns:        p.config.NumConns,
		Version:         p.cluster.NegotiatedVersion,
		Auth:            p.config.Auth,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	p.sessions.Store("", sess) // No keyspace

	p.listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}

	p.wp = &workerPool{
		WorkerFunc:      serveRequest,
		MaxWorkersCount: 2048, // TODO: Max count?
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
		ctx:                 p.ctx,
		proxy:               p,
		preparedSystemQuery: make(map[[16]byte]interface{}),
		preparedIdempotence: make(map[[16]byte]bool),
	}
	cl.conn = proxycore.NewConn(conn, cl)
	cl.conn.Start()
}

func (p *Proxy) maybeCreateSession(keyspace string) error {
	p.sessMu.Lock()
	defer p.sessMu.Unlock()
	if _, ok := p.sessions.Load(keyspace); !ok {
		sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
			ReconnectPolicy: p.config.ReconnectPolicy,
			NumConns:        p.config.NumConns,
			Version:         p.cluster.NegotiatedVersion,
			Auth:            p.config.Auth,
			Keyspace:        keyspace,
		})
		if err != nil {
			return err
		}
		p.sessions.Store(keyspace, sess)
	}
	return nil
}

func (p *Proxy) newQueryPlan() proxycore.QueryPlan {
	return p.lb.NewQueryPlan()
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
	encoded, err := proxycore.EncodeType(dt, p.cluster.NegotiatedVersion, val)
	if err != nil {
		p.logger.Fatal("unable to encode type", zap.Error(err))
	}
	return encoded
}

type client struct {
	ctx                 context.Context
	proxy               *Proxy
	conn                *proxycore.Conn
	keyspace            string
	preparedSystemQuery map[[16]byte]interface{}
	preparedIdempotence map[[16]byte]bool
}

func (c *client) Receive(reader io.Reader) error {
	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		}
		return err
	}

	if raw.Header.Version > primitive.ProtocolVersion4 {
		c.send(raw.Header, &message.ProtocolError{ErrorMessage: "Invalid or unsupported protocol version"})
		return nil
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
		c.handlePrepare(raw, msg)
	case *partialExecute:
		c.handleExecute(raw, msg)
	case *partialQuery:
		c.handleQuery(raw, msg)
	default:
		c.send(raw.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}

	return nil
}

func (c *client) execute(raw *frame.RawFrame, idempotent bool) {
	if sess, ok := c.proxy.sessions.Load(c.keyspace); ok {
		req := &request{
			client:     c,
			session:    sess.(*proxycore.Session),
			idempotent: idempotent,
			stream:     raw.Header.StreamId,
			qp:         c.proxy.newQueryPlan(),
			raw:        raw,
			err:        make(chan error),
			res:        make(chan *frame.RawFrame),
		}

		if !c.proxy.wp.Serve(req) {
			c.send(raw.Header, &message.Overloaded{ErrorMessage: "Proxy is overloaded"})
		}
	} else {
		c.send(raw.Header, &message.ServerError{ErrorMessage: "Attempted to use invalid keyspace"})
	}
}

func (c *client) handlePrepare(raw *frame.RawFrame, msg *message.Prepare) {
	keyspace := c.keyspace
	if len(msg.Keyspace) != 0 {
		keyspace = msg.Keyspace
	}
	handled, idempotent, stmt := parse(keyspace, msg.Query)

	if handled {
		hdr := raw.Header

		switch s := stmt.(type) {
		case *selectStatement:
			if s.table == "local" {
				if _, columns, err := c.handleSystemLocalOrPeersQuery(s, c.proxy.localRow, systemLocalColumns, 1); err != nil {
					c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
				} else {
					hash := md5.Sum([]byte(msg.Query + keyspace))
					c.send(hdr, &message.PreparedResult{
						PreparedQueryId: hash[:],
						ResultMetadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
					})
					c.preparedSystemQuery[hash] = stmt
				}
				c.preparedSystemQuery[md5.Sum([]byte(msg.Query+keyspace))] = stmt
			} else if s.table == "peers" {
				if _, columns, err := c.handleSystemLocalOrPeersQuery(s, nil, systemPeersColumns, 0); err != nil {
					c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
				} else {
					hash := md5.Sum([]byte(msg.Query + keyspace))
					c.send(hdr, &message.PreparedResult{
						PreparedQueryId: hash[:],
						ResultMetadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
					})
					c.preparedSystemQuery[hash] = stmt
				}
			} else {
				c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
			}
		case *useStatement:
			hash := md5.Sum([]byte(msg.Query))
			c.preparedSystemQuery[hash] = stmt
			c.send(hdr, &message.PreparedResult{
				PreparedQueryId: hash[:],
			})
		case *errorSelectStatement:
			c.send(hdr, &message.Invalid{ErrorMessage: s.err.Error()})
		default:
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
		}
	} else {
		c.preparedIdempotence[md5.Sum([]byte(msg.Query))] = idempotent
		c.execute(raw, true) // Prepared statements can be retried themselves
	}
}

func (c *client) handleExecute(raw *frame.RawFrame, msg *partialExecute) {
	var hash [16]byte
	copy(hash[:], msg.queryId)

	if stmt, ok := c.preparedSystemQuery[hash]; ok {
		c.handleSystemQuery(raw.Header, stmt)
	} else {
		c.execute(raw, c.preparedIdempotence[hash])
	}
}

func (c *client) handleQuery(raw *frame.RawFrame, msg *partialQuery) {
	handled, idempotent, stmt := parse(c.keyspace, msg.query)

	c.proxy.logger.Info("handling query", zap.String("query", msg.query), zap.Int16("stream", raw.Header.StreamId))

	if handled {
		c.handleSystemQuery(raw.Header, stmt)
	} else {
		c.execute(raw, idempotent)
	}
}

func (c *client) columnValue(values map[string]message.Column, name string, table string) message.Column {
	var val message.Column
	var ok bool
	if val, ok = values[name]; !ok {
		if name == "rpc_address" && table == "local" {
			switch addr := c.conn.LocalAddr().(type) {
			case *net.TCPAddr:
				val, _ = proxycore.EncodeType(datatype.Inet, c.proxy.cluster.NegotiatedVersion, addr.IP)
			}
		}
	}
	return val
}

func (c *client) handleSystemQuery(hdr *frame.Header, stmt interface{}) {
	switch s := stmt.(type) {
	case *selectStatement:
		if s.table == "local" {
			if row, columns, err := c.handleSystemLocalOrPeersQuery(s, c.proxy.localRow, systemLocalColumns, 1); err != nil {
				c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
			} else {
				c.send(hdr, &message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: int32(len(columns)),
						Columns:     columns,
					},
					Data: []message.Row{row},
				})
			}
		} else if s.table == "peers" {
			if _, columns, err := c.handleSystemLocalOrPeersQuery(s, nil, systemPeersColumns, 0); err != nil {
				c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
			} else {
				c.send(hdr, &message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: int32(len(columns)),
						Columns:     columns,
					},
				})
			}
		} else {
			c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
		}
	case *useStatement:
		if err := c.proxy.maybeCreateSession(s.keyspace); err != nil {
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy unable to create new session for keyspace"})
		} else {
			c.keyspace = s.keyspace
			c.send(hdr, &message.VoidResult{})
		}
	case *errorSelectStatement:
		c.send(hdr, &message.Invalid{ErrorMessage: s.err.Error()})
	default:
		c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
	}
}

func (c *client) handleSystemLocalOrPeersQuery(stmt *selectStatement, values map[string]message.Column,
	systemColumns []*message.ColumnMetadata, count int) (row message.Row, columns []*message.ColumnMetadata, err error) {
	if _, ok := stmt.selectors[0].(*starSelector); ok {
		for _, column := range systemColumns {
			val := c.columnValue(values, column.Name, stmt.table)
			row = append(row, val)
		}
		columns = systemColumns
	} else {
		for _, selector := range stmt.selectors {
			val, column, err := c.handleSelector(selector, values, systemColumns, count, stmt.table)
			if err != nil {
				return nil, nil, err
			}
			row = append(row, val)
			columns = append(columns, column)
		}
	}

	return row, columns, err

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
			return c.columnValue(c.proxy.localRow, column.Name, table), column, nil
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
	_ = c.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *client) Closing(err error) {
	_ = err
}
