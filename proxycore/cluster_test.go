package proxycore

import (
	"context"
	"cql-proxy/parser"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"log"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type mockClient struct {
	id         uint64
	server     *mockServer
	conn       *Conn
	keyspace   string
	registered int32
	events     chan message.Event
}

type mockHost struct {
	ip     string
	port   int
	hostID *primitive.UUID
}

func (h mockHost) String() string {
	return net.JoinHostPort(h.ip, strconv.Itoa(h.port))
}

func (h mockHost) equal(o mockHost) bool {
	return h.ip == o.ip && h.port == o.port
}

var (
	mockSchemaVersion, _ = primitive.ParseUuid("4f2b29e6-59b5-4e2d-8fd6-01e32e67f0d7")
)

func newMockClient(id uint64, server *mockServer) *mockClient {
	return &mockClient{
		id:     id,
		server: server,
		events: make(chan message.Event),
	}
}

func removeHost(hosts []mockHost, host mockHost) []mockHost {
	for i, h := range hosts {
		if h.equal(host) {
			return append(hosts[:i], hosts[i+1:]...)
		}
	}
	return hosts
}

func (c *mockClient) Receive(reader io.Reader) error {
	frm, err := codec.DecodeFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			//c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		}
		return err
	}

	if frm.Header.Version > c.server.maxVersion {
		c.send(frm.Header, &message.ProtocolError{ErrorMessage: "Invalid or unsupported protocol version"})
		return nil
	}

	switch msg := frm.Body.Message.(type) {
	case *message.Options:
		c.send(frm.Header, &message.Supported{Options: map[string][]string{"CQL_VERSION": {"3.4.0"}, "COMPRESSION": {}}})
	case *message.Startup:
		c.send(frm.Header, &message.Ready{})
	case *message.Register:
		atomic.StoreInt32(&c.registered, int32(frm.Header.Version))
		c.send(frm.Header, &message.Ready{})
	case *message.Query:
		c.handleQuery(frm.Header, msg)
	default:
		c.send(frm.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}

	return nil
}

func makeSystemLocalValues(version primitive.ProtocolVersion, address string, hostID, schemaVersion *primitive.UUID) map[string]message.Column {
	ip := net.ParseIP(address)
	values := makeSystemValues(version, ip, schemaVersion, hostID)
	values["key"] = encodeTypeFatal(version, datatype.Varchar, "local")
	values["partitioner"] = encodeTypeFatal(version, datatype.Varchar, "")
	values["cluster_name"] = encodeTypeFatal(version, datatype.Varchar, "cql-proxy")
	values["cql_version"] = encodeTypeFatal(version, datatype.Varchar, "3.4.5")
	values["native_protocol_version"] = encodeTypeFatal(version, datatype.Varchar, version.String())
	return values
}

func makeSystemPeerValues(version primitive.ProtocolVersion, address string, hostID, schemaVersion *primitive.UUID) map[string]message.Column {
	ip := net.ParseIP(address)
	values := makeSystemValues(version, ip, schemaVersion, hostID)
	values["peer"] = encodeTypeFatal(version, datatype.Inet, ip)
	return values
}

func makeSystemValues(version primitive.ProtocolVersion, address net.IP, hostID, schemaVersion *primitive.UUID) map[string]message.Column {
	return map[string]message.Column{
		"rpc_address":     encodeTypeFatal(version, datatype.Inet, address),
		"data_center":     encodeTypeFatal(version, datatype.Varchar, "dc1"),
		"rack":            encodeTypeFatal(version, datatype.Varchar, "rack1"),
		"tokens":          encodeTypeFatal(version, datatype.NewListType(datatype.Varchar), []string{"0"}),
		"release_version": encodeTypeFatal(version, datatype.Varchar, "3.11.10"),
		"host_id":         encodeTypeFatal(version, datatype.Uuid, hostID),
		"schema_version":  encodeTypeFatal(version, datatype.Uuid, schemaVersion), // TODO: Make this match the downstream cluster(s)
	}
}

func encodeTypeFatal(version primitive.ProtocolVersion, dt datatype.DataType, val interface{}) []byte {
	encoded, err := EncodeType(dt, version, val)
	if err != nil {
		panic("unable to encode type")
	}
	return encoded
}

func (c mockClient) filterValues(version primitive.ProtocolVersion, stmt *parser.SelectStatement,
	columns []*message.ColumnMetadata, vals map[string]message.Column, count int) (row []message.Column, err error) {
	return parser.FilterValues(stmt, columns, func(name string) (value message.Column, err error) {
		if val, ok := vals[name]; ok {
			return val, nil
		} else if name == parser.CountValueName {
			return EncodeType(datatype.Int, version, count)
		} else {
			return nil, fmt.Errorf("no column value for %s", name)
		}
	})
}

func (c *mockClient) handleQuery(hdr *frame.Header, msg *message.Query) {
	handled, _, stmt := parser.Parse(c.keyspace, msg.Query)

	log.Println(msg.Query)

	if handled {
		switch s := stmt.(type) {
		case *parser.SelectStatement:
			if s.Table == "local" {
				vals := makeSystemLocalValues(hdr.Version, c.server.local.ip, c.server.local.hostID, mockSchemaVersion)
				if columns, err := parser.FilterColumns(s, parser.SystemLocalColumns); err != nil {
					c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
				} else if row, err := c.filterValues(hdr.Version, s, parser.SystemLocalColumns, vals, 1); err != nil {
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
					peers := c.server.copyPeers()
					for _, peer := range peers {
						vals := makeSystemPeerValues(hdr.Version, peer.ip, peer.hostID, mockSchemaVersion)
						if row, err := c.filterValues(hdr.Version, s, parser.SystemPeersColumns, vals, len(peers)); err != nil {
							c.send(hdr, &message.Invalid{ErrorMessage: err.Error()})
							return
						} else {
							data = append(data, row)
						}
					}
					c.send(hdr, &message.RowsResult{
						Metadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
						Data: data,
					})
				}
			} else {
				c.send(hdr, &message.Invalid{ErrorMessage: "Doesn't exist"})
			}
		case *parser.UseStatement:
			c.keyspace = s.Keyspace
			c.send(hdr, &message.VoidResult{})
		case *parser.ErrorSelectStatement:
			c.send(hdr, &message.Invalid{ErrorMessage: s.Err.Error()})
		default:
			c.send(hdr, &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"})
		}
	} else {
		c.send(hdr, &message.RowsResult{
			Metadata: &message.RowsMetadata{
				ColumnCount: 0,
			},
			Data: message.RowSet{},
		})
	}
}

func (c *mockClient) send(hdr *frame.Header, msg message.Message) {
	_ = c.conn.Write(SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c mockClient) Closing(err error) {
	c.server.clients.Delete(c.id)
}

type mockServer struct {
	cancel      context.CancelFunc
	clients     sync.Map
	clientIdGen uint64
	maxVersion  primitive.ProtocolVersion
	local       mockHost
	peers       []mockHost
	mu          sync.Mutex
}

func (s *mockServer) add(host mockHost) {
	if host.equal(s.local) {
		return
	}
	s.mu.Lock()
	s.peers = append(s.peers, host)
	s.mu.Unlock()
	event := &message.TopologyChangeEvent{
		ChangeType: primitive.TopologyChangeTypeNewNode,
		Address: &primitive.Inet{
			Addr: net.ParseIP(host.ip),
			Port: int32(host.port),
		},
	}
	s.clients.Range(func(_, value interface{}) bool {
		cl := value.(*mockClient)
		cl.events <- event
		return true
	})
}

func (s *mockServer) remove(host mockHost) {
	if host.equal(s.local) {
		return
	}
	s.mu.Lock()
	s.peers = removeHost(s.peers, host)
	s.mu.Unlock()
	event := &message.TopologyChangeEvent{
		ChangeType: primitive.TopologyChangeTypeRemovedNode,
		Address: &primitive.Inet{
			Addr: net.ParseIP(host.ip),
			Port: int32(host.port),
		},
	}
	s.clients.Range(func(_, value interface{}) bool {
		cl := value.(*mockClient)
		cl.events <- event
		return true
	})
}

func (s *mockServer) copyPeers() []mockHost {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := make([]mockHost, len(s.peers))
	copy(c, s.peers)
	return c
}

func (s *mockServer) serve(ctx context.Context, maxVersion primitive.ProtocolVersion, local mockHost, peers []mockHost) error {

	var listener net.Listener
	var err error
	for {
		listener, err = net.Listen("tcp", net.JoinHostPort(local.ip, strconv.Itoa(local.port)))
		if err == nil || !errors.Is(err, syscall.EADDRINUSE) {
			break
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-timer.C:

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err != nil {
		return err
	}

	ctx, s.cancel = context.WithCancel(ctx)

	s.maxVersion = maxVersion
	s.local = local
	s.peers = removeHost(peers, local)

	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				break
			}
			id := atomic.AddUint64(&s.clientIdGen, 1)
			cl := newMockClient(id, s)
			cl.conn = NewConn(c, cl)
			go func(cl *mockClient) {
				done := false
				for !done {
					select {
					case event := <-cl.events:
						registered := atomic.LoadInt32(&cl.registered)
						if registered != 0 {
							cl.conn.Write(SenderFunc(func(writer io.Writer) error {
								return codec.EncodeFrame(frame.NewFrame(primitive.ProtocolVersion(registered), -1, event), writer)
							}))
						}
					case <-cl.conn.IsClosed():
						done = true
					}
				}
			}(cl)
			s.clients.Store(id, cl)
			cl.conn.Start()

		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
			s.clients.Range(func(key, value interface{}) bool {
				cl := value.(*mockClient)
				cl.conn.Close()
				return true
			})
		}
	}()

	return nil
}

type mockCluster struct {
	startHostID *primitive.UUID
	startIP     net.IP
	port        int
	hosts       []mockHost
	servers     map[string]*mockServer
}

func newMockCluster(startIP net.IP) *mockCluster {
	hostID, _ := primitive.ParseUuid("b3bca296-5bb7-411d-b875-67c33fe10000")
	return &mockCluster{
		startHostID: hostID,
		startIP:     startIP,
		port:        9042,
		servers:     make(map[string]*mockServer),
	}
}

func (c mockCluster) generate(n int) (host mockHost) {
	host.port = c.port
	host.hostID = &primitive.UUID{}
	new(big.Int).Add(new(big.Int).SetBytes(c.startHostID[:]), big.NewInt(int64(n))).FillBytes(host.hostID[:])
	ip := make(net.IP, net.IPv6len)
	new(big.Int).Add(new(big.Int).SetBytes(c.startIP), big.NewInt(int64(n))).FillBytes(ip)
	host.ip = ip.String()
	return host
}

func (c *mockCluster) add(ctx context.Context, n int) {
	host := c.generate(n)
	for _, h := range c.hosts {
		if h.equal(host) {
			return
		}
	}
	c.maybeStart(ctx, host)
	c.hosts = append(c.hosts, host)
	for _, s := range c.servers {
		s.add(host)
	}
}

func (c *mockCluster) maybeStart(ctx context.Context, host mockHost) {
	key := host.String()
	if _, ok := c.servers[key]; !ok {
		server := &mockServer{}
		server.serve(ctx, primitive.ProtocolVersion4, host, c.hosts)
		c.servers[key] = server
	}
}

func (c *mockCluster) start(ctx context.Context, n int) {
	c.maybeStart(ctx, c.generate(n))
}

func (c *mockCluster) remove(n int) {
	host := c.generate(n)
	removed := removeHost(c.hosts, host)
	if len(removed) == len(c.hosts) {
		return
	}
	for _, s := range c.servers {
		s.remove(host)
	}
	c.maybeStop(host)
}

func (c *mockCluster) maybeStop(host mockHost) {
	key := host.String()
	if s, ok := c.servers[key]; ok {
		s.cancel()
		delete(c.servers, key)
	}
}

func (c *mockCluster) stop(n int) {
	c.maybeStop(c.generate(n))
}

func TestConnectCluster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := newMockCluster(net.ParseIP("127.0.0.0"))

	c.add(ctx, 1)
	c.add(ctx, 2)
	c.add(ctx, 3)

	logger, _ := zap.NewDevelopment()
	cluster, err := ConnectCluster(ctx, ClusterConfig{
		Version:         primitive.ProtocolVersion4,
		Resolver:        NewResolver("127.0.0.1:9042"),
		ReconnectPolicy: NewReconnectPolicy(),
		Logger:          logger,
	})
	require.NoError(t, err)

	c.stop(1)
	c.start(ctx, 1)
	c.stop(2)
	c.start(ctx, 2)
	c.stop(3)
	c.start(ctx, 3)
	c.stop(2)

	select {
	case <-ctx.Done(): // TODO: Make not infinitely blocking
	}

	_ = cluster
}
