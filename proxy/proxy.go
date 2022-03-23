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

package proxy

import (
	"bytes"
	"context"
	"crypto"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datastax/cql-proxy/parser"
	"github.com/datastax/cql-proxy/proxycore"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
)

var (
	encodedOneValue, _  = proxycore.EncodeType(datatype.Int, primitive.ProtocolVersion4, 1)
	encodedZeroValue, _ = proxycore.EncodeType(datatype.Int, primitive.ProtocolVersion4, 0)
)

var ErrProxyClosed = errors.New("proxy closed")

const preparedIdSize = 16

type Config struct {
	Version           primitive.ProtocolVersion
	MaxVersion        primitive.ProtocolVersion
	Auth              proxycore.Authenticator
	Resolver          proxycore.EndpointResolver
	ReconnectPolicy   proxycore.ReconnectPolicy
	RetryPolicy       RetryPolicy
	NumConns          int
	Logger            *zap.Logger
	HeartBeatInterval time.Duration
	IdleTimeout       time.Duration
	RPCAddr           string
	DataCenter        string
	Peers             []string
	// PreparedCache a cache that stores prepared queries. If not set it uses the default implementation with a max
	// capacity of ~100MB.
	PreparedCache proxycore.PreparedCache
}

type Proxy struct {
	ctx                 context.Context
	config              Config
	logger              *zap.Logger
	listener            *net.TCPListener
	cluster             *proxycore.Cluster
	sessions            [primitive.ProtocolVersionDse2 + 1]sync.Map // Cache sessions per protocol version
	sessMu              sync.Mutex
	schemaEventClients  sync.Map
	preparedCache       proxycore.PreparedCache
	preparedIdempotence sync.Map
	clientIdGen         uint64
	lb                  proxycore.LoadBalancer
	systemLocalValues   map[string]message.Column
	closed              chan struct{}
	closingMu           sync.Mutex
	localNode           *node
	nodes               []*node
}

type node struct {
	Addr       *net.TCPAddr
	DataCenter string
	token      string
	hostId     primitive.UUID
}

func (p *Proxy) OnEvent(event proxycore.Event) {
	switch evt := event.(type) {
	case *proxycore.SchemaChangeEvent:
		frm := frame.NewFrame(p.cluster.NegotiatedVersion, -1, evt.Message)
		p.schemaEventClients.Range(func(key, value interface{}) bool {
			cl := value.(*client)
			err := cl.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
				return codec.EncodeFrame(frm, writer)
			}))
			cl.conn.LocalAddr()
			if err != nil {
				p.logger.Error("unable to send schema change event",
					zap.Stringer("client", cl.conn.RemoteAddr()),
					zap.Uint64("id", cl.id),
					zap.Error(err))
				_ = cl.conn.Close()
			}
			return true
		})
	}
}

func NewProxy(ctx context.Context, config Config) *Proxy {
	if config.Version == 0 {
		config.Version = primitive.ProtocolVersion4
	}
	if config.MaxVersion == 0 {
		config.MaxVersion = primitive.ProtocolVersion4
	}
	if config.RetryPolicy == nil {
		config.RetryPolicy = NewDefaultRetryPolicy()
	}
	return &Proxy{
		ctx:    ctx,
		config: config,
		logger: proxycore.GetOrCreateNopLogger(config.Logger),
		closed: make(chan struct{}),
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

	p.preparedCache, err = getOrCreateDefaultPreparedCache(p.config.PreparedCache)
	if err != nil {
		return fmt.Errorf("unable to create prepared cache %w", err)
	}

	p.cluster, err = proxycore.ConnectCluster(p.ctx, proxycore.ClusterConfig{
		Version:           p.config.Version,
		Auth:              p.config.Auth,
		Resolver:          p.config.Resolver,
		ReconnectPolicy:   p.config.ReconnectPolicy,
		HeartBeatInterval: p.config.HeartBeatInterval,
		IdleTimeout:       p.config.IdleTimeout,
		Logger:            p.logger,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	err = p.cluster.Listen(p)
	if err != nil {
		return fmt.Errorf("unable to register to listen for schema events %w", err)
	}

	err = p.buildNodes()
	if err != nil {
		return fmt.Errorf("unable to build node information %w", err)
	}

	p.buildLocalRow()

	if len(p.config.DataCenter) > 0 {
		p.lb = proxycore.NewLocalDCOnlyLoadBalancer(p.config.DataCenter)
	} else {
		p.lb = proxycore.NewRoundRobinLoadBalancer()
	}

	err = p.cluster.Listen(p.lb)
	if err != nil {
		return err
	}

	sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
		ReconnectPolicy:   p.config.ReconnectPolicy,
		NumConns:          p.config.NumConns,
		Version:           p.cluster.NegotiatedVersion,
		Auth:              p.config.Auth,
		HeartBeatInterval: p.config.HeartBeatInterval,
		IdleTimeout:       p.config.IdleTimeout,
		PreparedCache:     p.preparedCache,
		Logger:            p.logger,
	})

	if err != nil {
		return fmt.Errorf("unable to connect session %w", err)
	}

	p.sessions[p.cluster.NegotiatedVersion].Store("", sess) // No keyspace

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return err
	}
	p.listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	p.logger.Info("proxy is listening", zap.Stringer("address", p.listener.Addr()))

	return nil
}

func (p *Proxy) Serve() error {
	for {
		conn, err := p.listener.AcceptTCP()
		if err != nil {
			select {
			case <-p.closed:
				return ErrProxyClosed
			default:
				return err
			}
		}
		p.handle(conn)
	}
}

func (p *Proxy) Close() error {
	p.closingMu.Lock()
	defer p.closingMu.Unlock()
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
	return p.listener.Close()
}

func (p *Proxy) Ready() bool {
	return true
}

func (p *Proxy) OutageDuration() time.Duration {
	return p.cluster.OutageDuration()
}

func (p *Proxy) handle(conn *net.TCPConn) {
	if err := conn.SetKeepAlive(false); err != nil {
		p.logger.Warn("failed to disable keepalive on connection", zap.Error(err))
	}

	if err := conn.SetNoDelay(true); err != nil {
		p.logger.Warn("failed to set TCP_NODELAY on connection", zap.Error(err))
	}

	cl := &client{
		ctx:                 p.ctx,
		proxy:               p,
		id:                  atomic.AddUint64(&p.clientIdGen, 1),
		preparedSystemQuery: make(map[[preparedIdSize]byte]interface{}),
	}
	cl.conn = proxycore.NewConn(conn, cl)
	cl.conn.Start()
}

func (p *Proxy) maybeCreateSession(version primitive.ProtocolVersion, keyspace string) (*proxycore.Session, error) {
	p.sessMu.Lock()
	defer p.sessMu.Unlock()
	if cachedSession, ok := p.sessions[version].Load(keyspace); ok {
		return cachedSession.(*proxycore.Session), nil
	} else {
		sess, err := proxycore.ConnectSession(p.ctx, p.cluster, proxycore.SessionConfig{
			ReconnectPolicy:   p.config.ReconnectPolicy,
			NumConns:          p.config.NumConns,
			Version:           version,
			Auth:              p.config.Auth,
			PreparedCache:     p.preparedCache,
			Keyspace:          keyspace,
			HeartBeatInterval: p.config.HeartBeatInterval,
			IdleTimeout:       p.config.IdleTimeout,
			Logger:            p.logger,
		})
		if err != nil {
			return nil, err
		}
		p.sessions[version].Store(keyspace, sess)
		return sess, nil
	}
}

func (p *Proxy) findSession(version primitive.ProtocolVersion, keyspace string) (*proxycore.Session, error) {
	if s, ok := p.sessions[version].Load(keyspace); ok {
		return s.(*proxycore.Session), nil
	} else {
		return p.maybeCreateSession(version, keyspace)
	}
}

func (p *Proxy) newQueryPlan() proxycore.QueryPlan {
	return p.lb.NewQueryPlan()
}

var (
	schemaVersion, _ = primitive.ParseUuid("4f2b29e6-59b5-4e2d-8fd6-01e32e67f0d7")
)

func (p *Proxy) buildNodes() (err error) {
	numPeers := len(p.config.Peers)
	nodes := make([]*node, 0, numPeers+1)

	var localRpcAddr *net.TCPAddr
	if len(p.config.RPCAddr) > 0 {
		localRpcAddr, err = net.ResolveTCPAddr("tcp", p.config.RPCAddr)
		if err != nil {
			return fmt.Errorf("invalid RPC address: %w", err)
		}
	}

	p.localNode = &node{
		Addr:       localRpcAddr,
		DataCenter: p.config.DataCenter,
		token:      strconv.FormatInt(math.MinInt64, 10),
	}
	nodes = append(nodes, p.localNode)

	for _, peer := range p.config.Peers {
		pair := strings.Split(peer, ";")
		dataCenter := p.config.DataCenter
		if len(pair) > 1 {
			dataCenter = pair[1]
		}
		peerAddr, err := net.ResolveTCPAddr("tcp", pair[0])
		if err != nil {
			return fmt.Errorf("invalid peer address: %w", err)
		}
		nodes = append(nodes, &node{
			Addr:       peerAddr,
			DataCenter: dataCenter,
		})
	}

	if len(nodes) > 1 {
		sort.Slice(nodes, func(i, j int) bool {
			a := nodes[i]
			b := nodes[j]
			res := bytes.Compare(a.Addr.IP, b.Addr.IP)
			if res < 0 ||
				res == 0 && a.Addr.Port < b.Addr.Port ||
				a.Addr.Port == b.Addr.Port && a.Addr.Zone < b.Addr.Zone ||
				a.Addr.Zone == b.Addr.Zone && a.DataCenter < b.DataCenter {
				return true
			}
			return false
		})

		var numTokens big.Int
		numTokens.SetUint64(math.MaxUint64/uint64(numPeers+1) + 1)

		start := big.NewInt(math.MinInt64)

		for _, n := range nodes {
			n.token = start.Text(10)
			n.hostId = nameBasedUUID(n.Addr.String())
			start.Add(start, &numTokens)
		}
	}

	p.nodes = nodes

	return nil
}

func (p *Proxy) buildLocalRow() {
	p.systemLocalValues = map[string]message.Column{
		"key":                     p.encodeTypeFatal(datatype.Varchar, "local"),
		"data_center":             p.encodeTypeFatal(datatype.Varchar, p.config.DataCenter),
		"rack":                    p.encodeTypeFatal(datatype.Varchar, "rack1"),
		"tokens":                  p.encodeTypeFatal(datatype.NewListType(datatype.Varchar), []string{p.localNode.token}),
		"release_version":         p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.ReleaseVersion),
		"partitioner":             p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.Partitioner),
		"cluster_name":            p.encodeTypeFatal(datatype.Varchar, "cql-proxy"),
		"cql_version":             p.encodeTypeFatal(datatype.Varchar, p.cluster.Info.CQLVersion),
		"schema_version":          p.encodeTypeFatal(datatype.Uuid, schemaVersion), // TODO: Make this match the downstream cluster(s)
		"native_protocol_version": p.encodeTypeFatal(datatype.Varchar, p.cluster.NegotiatedVersion.String()),
	}
}

func (p *Proxy) encodeTypeFatal(dt datatype.DataType, val interface{}) []byte {
	encoded, err := proxycore.EncodeType(dt, p.cluster.NegotiatedVersion, val)
	if err != nil {
		p.logger.Fatal("unable to encode type", zap.Error(err))
	}
	return encoded
}

// isIdempotent checks whether a prepared ID is idempotent.
// If the proxy receives a query that it's never prepared then this will also return false.
func (p *Proxy) isIdempotent(id []byte) bool {
	if val, ok := p.preparedIdempotence.Load(preparedIdKey(id)); !ok {
		// This should only happen if the proxy has never had a "PREPARE" request for this query ID.
		p.logger.Error("unable to determine if prepared statement is idempotent",
			zap.String("preparedID", hex.EncodeToString(id)))
		return false
	} else {
		return val.(bool)
	}
}

// maybeStorePreparedIdempotence stores the idempotence of a "PREPARE" request's query.
// This information is used by future "EXECUTE" requests when they need to be retried.
func (p *Proxy) maybeStorePreparedIdempotence(raw *frame.RawFrame, msg message.Message) {
	if prepareMsg, ok := msg.(*message.Prepare); ok && raw.Header.OpCode == primitive.OpCodeResult { // Prepared result
		frm, err := codec.ConvertFromRawFrame(raw)
		if err != nil {
			p.logger.Error("error attempting to decode prepared result message")
		} else if _, ok = frm.Body.Message.(*message.PreparedResult); !ok { // TODO: Use prepared type data to disambiguate idempotency
			p.logger.Error("expected prepared result message, but got something else")
		} else {
			idempotent, err := parser.IsQueryIdempotent(prepareMsg.Query)
			if err != nil {
				p.logger.Error("error parsing query for idempotence", zap.Error(err))
			} else if result, ok := frm.Body.Message.(*message.PreparedResult); ok {
				p.preparedIdempotence.Store(preparedIdKey(result.PreparedQueryId), idempotent)
			} else {
				p.logger.Error("expected prepared result, but got some other type of message",
					zap.Stringer("type", reflect.TypeOf(frm.Body.Message)))
			}
		}
	}
}

type client struct {
	ctx                 context.Context
	proxy               *Proxy
	conn                *proxycore.Conn
	keyspace            string
	id                  uint64
	preparedSystemQuery map[[16]byte]interface{}
}

func (c *client) Receive(reader io.Reader) error {
	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		}
		return err
	}

	if raw.Header.Version > c.proxy.config.MaxVersion || raw.Header.Version < primitive.ProtocolVersion3 {
		c.send(raw.Header, &message.ProtocolError{
			ErrorMessage: fmt.Sprintf("Invalid or unsupported protocol version %d", raw.Header.Version),
		})
		return nil
	}

	body, err := codec.DecodeBody(raw.Header, bytes.NewReader(raw.Body))
	if err != nil {
		c.proxy.logger.Error("unable to decode body", zap.Error(err))
		return err
	}

	switch msg := body.Message.(type) {
	case *message.Options:
		c.send(raw.Header, &message.Supported{Options: map[string][]string{
			"CQL_VERSION": {c.proxy.cluster.Info.CQLVersion},
			"COMPRESSION": {},
		}})
	case *message.Startup:
		c.send(raw.Header, &message.Ready{})
	case *message.Register:
		for _, t := range msg.EventTypes {
			if t == primitive.EventTypeSchemaChange {
				c.proxy.schemaEventClients.Store(c.id, c)
			}
		}
		c.send(raw.Header, &message.Ready{})
	case *message.Prepare:
		c.handlePrepare(raw, msg)
	case *partialExecute:
		c.handleExecute(raw, msg)
	case *partialQuery:
		c.handleQuery(raw, msg)
	case *partialBatch:
		c.execute(raw, notDetermined, c.keyspace, msg)
	default:
		c.send(raw.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}

	return nil
}

func (c *client) execute(raw *frame.RawFrame, state idempotentState, keyspace string, msg message.Message) {
	if sess, err := c.proxy.findSession(raw.Header.Version, c.keyspace); err == nil {
		req := &request{
			client:   c,
			session:  sess,
			state:    state,
			msg:      msg,
			keyspace: keyspace,
			done:     false,
			stream:   raw.Header.StreamId,
			qp:       c.proxy.newQueryPlan(),
			raw:      raw,
		}
		req.Execute(true)
	} else {
		c.send(raw.Header, &message.ServerError{ErrorMessage: "Attempted to use invalid keyspace"})
	}
}

func (c *client) handlePrepare(raw *frame.RawFrame, msg *message.Prepare) {
	c.proxy.logger.Debug("handling prepare", zap.String("query", msg.Query), zap.Int16("stream", raw.Header.StreamId))

	keyspace := c.keyspace
	if len(msg.Keyspace) != 0 {
		keyspace = msg.Keyspace
	}
	handled, stmt, err := parser.IsQueryHandled(parser.IdentifierFromString(keyspace), msg.Query)

	if handled {
		hdr := raw.Header

		if err != nil {
			c.proxy.logger.Error("error parsing query to see if it's handled", zap.Error(err))
			c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
		} else {
			switch s := stmt.(type) {
			case *parser.SelectStatement:
				if systemColumns, ok := parser.SystemColumnsByName[s.Table]; ok {
					if columns, err := parser.FilterColumns(s, systemColumns); err != nil {
						c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
					} else {
						id := md5.Sum([]byte(msg.Query + keyspace))
						c.send(hdr, &message.PreparedResult{
							PreparedQueryId: id[:],
							ResultMetadata: &message.RowsMetadata{
								ColumnCount: int32(len(columns)),
								Columns:     columns,
							},
						})
						c.preparedSystemQuery[id] = stmt
					}
				} else {
					c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
				}
			case *parser.UseStatement:
				id := md5.Sum([]byte(msg.Query))
				c.preparedSystemQuery[id] = stmt
				c.send(hdr, &message.PreparedResult{
					PreparedQueryId: id[:],
				})
			default:
				c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
			}
		}

	} else {
		c.execute(raw, isIdempotent, keyspace, msg) // Prepared statements can be retried themselves
	}
}

func (c *client) handleExecute(raw *frame.RawFrame, msg *partialExecute) {
	id := preparedIdKey(msg.queryId)
	if stmt, ok := c.preparedSystemQuery[id]; ok {
		c.interceptSystemQuery(raw.Header, stmt)
	} else {
		c.execute(raw, notDetermined, "", msg)
	}
}

func (c *client) handleQuery(raw *frame.RawFrame, msg *partialQuery) {
	c.proxy.logger.Debug("handling query", zap.String("query", msg.query), zap.Int16("stream", raw.Header.StreamId))

	handled, stmt, err := parser.IsQueryHandled(parser.IdentifierFromString(c.keyspace), msg.query)

	if handled {
		if err != nil {
			c.proxy.logger.Error("error parsing query to see if it's handled", zap.Error(err))
			c.send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		} else {
			c.interceptSystemQuery(raw.Header, stmt)
		}
	} else {
		c.execute(raw, notDetermined, c.keyspace, msg)
	}
}

func (c *client) filterSystemLocalValues(stmt *parser.SelectStatement) (row []message.Column, err error) {
	return parser.FilterValues(stmt, parser.SystemLocalColumns, func(name string) (value message.Column, err error) {
		if name == "rpc_address" {
			var addr net.Addr
			if c.proxy.localNode.Addr != nil {
				addr = c.proxy.localNode.Addr
			} else {
				addr = c.conn.LocalAddr()
			}
			switch a := addr.(type) {
			case *net.TCPAddr:
				return proxycore.EncodeType(datatype.Inet, c.proxy.cluster.NegotiatedVersion, a.IP)
			default:
				return nil, errors.New("unhandled local address type")
			}
		} else if name == "host_id" {
			var addr net.Addr
			if c.proxy.localNode.Addr != nil {
				addr = c.proxy.localNode.Addr
			} else {
				addr = c.conn.LocalAddr()
			}
			return proxycore.EncodeType(datatype.Uuid, c.proxy.cluster.NegotiatedVersion, nameBasedUUID(addr.String()))
		} else if val, ok := c.proxy.systemLocalValues[name]; ok {
			return val, nil
		} else if name == parser.CountValueName {
			return encodedOneValue, nil
		} else {
			return nil, fmt.Errorf("no column value for %s", name)
		}
	})
}

func (c *client) filterSystemPeerValues(stmt *parser.SelectStatement, peer *node, peerCount int) (row []message.Column, err error) {
	return parser.FilterValues(stmt, parser.SystemPeersColumns, func(name string) (value message.Column, err error) {
		if name == "data_center" {
			return proxycore.EncodeType(datatype.Varchar, c.proxy.cluster.NegotiatedVersion, peer.DataCenter)
		} else if name == "host_id" {
			return proxycore.EncodeType(datatype.Uuid, c.proxy.cluster.NegotiatedVersion, peer.hostId)
		} else if name == "tokens" {
			return proxycore.EncodeType(datatype.NewListType(datatype.Varchar), c.proxy.cluster.NegotiatedVersion, []string{peer.token})
		} else if name == "peer" {
			return proxycore.EncodeType(datatype.Varchar, c.proxy.cluster.NegotiatedVersion, peer.Addr.IP.String())
		} else if name == "rpc_address" {
			return proxycore.EncodeType(datatype.Inet, c.proxy.cluster.NegotiatedVersion, peer.Addr.IP)
		} else if val, ok := c.proxy.systemLocalValues[name]; ok {
			return val, nil
		} else if name == parser.CountValueName {
			return proxycore.EncodeType(datatype.Int, c.proxy.cluster.NegotiatedVersion, peerCount)
		} else {
			return nil, fmt.Errorf("no column value for %s", name)
		}
	})
}

func (c *client) interceptSystemQuery(hdr *frame.Header, stmt interface{}) {
	switch s := stmt.(type) {
	case *parser.SelectStatement:
		if s.Table == "local" {
			if columns, err := parser.FilterColumns(s, parser.SystemLocalColumns); err != nil {
				c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
			} else if row, err := c.filterSystemLocalValues(s); err != nil {
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
		} else if s.Table == "peers" {
			if columns, err := parser.FilterColumns(s, parser.SystemPeersColumns); err != nil {
				c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
			} else {
				var data []message.Row
				for _, n := range c.proxy.nodes {
					if n != c.proxy.localNode {
						var row message.Row
						row, err = c.filterSystemPeerValues(s, n, len(c.proxy.nodes)-1)
						if err != nil {
							break
						}
						data = append(data, row)
					}
				}
				if err != nil {
					c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
				} else {
					c.send(hdr, &message.RowsResult{
						Metadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
						Data: data,
					})
				}
			}
		} else if columns, ok := parser.SystemColumnsByName[s.Table]; ok {
			c.send(hdr, &message.RowsResult{
				Metadata: &message.RowsMetadata{
					ColumnCount: int32(len(columns)),
					Columns:     columns,
				},
			})
		} else {
			c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
		}
	case *parser.UseStatement:
		if _, err := c.proxy.maybeCreateSession(hdr.Version, s.Keyspace); err != nil {
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy unable to create new session for keyspace"})
		} else {
			c.keyspace = s.Keyspace
			c.send(hdr, &message.SetKeyspaceResult{Keyspace: s.Keyspace})
		}
	default:
		c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
	}
}

func (c *client) send(hdr *frame.Header, msg message.Message) {
	_ = c.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *client) Closing(_ error) {
	c.proxy.schemaEventClients.Delete(c.id)
}

func getOrCreateDefaultPreparedCache(cache proxycore.PreparedCache) (proxycore.PreparedCache, error) {
	if cache == nil {
		return NewDefaultPreparedCache(1e8 / 256) // ~100MB with an average query size of 256 bytes
	}
	return cache, nil
}

// NewDefaultPreparedCache creates a new default prepared cache capping the max item capacity to `size`.
func NewDefaultPreparedCache(size int) (proxycore.PreparedCache, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &defaultPreparedCache{cache}, nil
}

type defaultPreparedCache struct {
	cache *lru.Cache
}

func (d defaultPreparedCache) Store(id string, entry *proxycore.PreparedEntry) {
	d.cache.Add(id, entry)
}

func (d defaultPreparedCache) Load(id string) (entry *proxycore.PreparedEntry, ok bool) {
	if val, ok := d.cache.Get(id); ok {
		return val.(*proxycore.PreparedEntry), true
	}
	return nil, false
}

func preparedIdKey(bytes []byte) [preparedIdSize]byte {
	var buf [preparedIdSize]byte
	copy(buf[:], bytes)
	return buf
}

func nameBasedUUID(name string) primitive.UUID {
	var uuid primitive.UUID
	m := crypto.MD5.New()
	_, _ = io.WriteString(m, name)
	hash := m.Sum(nil)
	for i := 0; i < len(uuid); i++ {
		uuid[i] = hash[i]
	}
	uuid[6] &= 0x0F
	uuid[6] |= 0x30
	uuid[8] &= 0x3F
	uuid[8] |= 0x80
	return uuid
}
