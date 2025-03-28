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
	"io"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/datastax/cql-proxy/parser"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var (
	mockSchemaVersion, _ = primitive.ParseUuid("4f2b29e6-59b5-4e2d-8fd6-01e32e67f0d7")
	mockHostID, _        = primitive.ParseUuid("0a9ca869-9031-4d86-8a17-647b9606f757")
)

type MockHost struct {
	IP     string
	Port   int
	HostID *primitive.UUID
}

func (h MockHost) String() string {
	return net.JoinHostPort(h.IP, strconv.Itoa(h.Port))
}

func (h MockHost) equal(o MockHost) bool {
	return h.IP == o.IP && h.Port == o.Port
}

type MockRequestHandler func(client *MockClient, frm *frame.Frame) message.Message

func MockDefaultOptionsHandler(_ *MockClient, _ *frame.Frame) message.Message {
	return &message.Supported{Options: map[string][]string{"CQL_VERSION": {"3.4.0"}, "COMPRESSION": {}}}
}

func MockDefaultStartupHandler(_ *MockClient, _ *frame.Frame) message.Message {
	return &message.Ready{}
}

func MockDefaultRegisterHandler(cl *MockClient, frm *frame.Frame) message.Message {
	cl.Register(frm.Header.Version)
	return &message.Ready{}
}

func MockDefaultQueryHandler(cl *MockClient, frm *frame.Frame) message.Message {
	if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
		return msg
	} else {
		return &message.RowsResult{
			Metadata: &message.RowsMetadata{
				ColumnCount: 0,
			},
			Data: message.RowSet{},
		}
	}
}

type MockRequestHandlers map[primitive.OpCode]MockRequestHandler

var DefaultMockRequestHandlers = MockRequestHandlers{
	primitive.OpCodeOptions:  MockDefaultOptionsHandler,
	primitive.OpCodeStartup:  MockDefaultStartupHandler,
	primitive.OpCodeRegister: MockDefaultRegisterHandler,
	primitive.OpCodeQuery:    MockDefaultQueryHandler,
}

func NewMockRequestHandlers(overrides MockRequestHandlers) MockRequestHandlers {
	handlers := make(MockRequestHandlers)
	for code, handler := range DefaultMockRequestHandlers {
		handlers[code] = handler
	}
	for code, handler := range overrides {
		handlers[code] = handler
	}
	return handlers
}

type MockClient struct {
	server     *MockServer
	conn       *Conn
	keyspace   string
	registered int32
	events     chan message.Event
}

func newMockClient(server *MockServer) *MockClient {
	return &MockClient{
		server: server,
		events: make(chan message.Event),
	}
}

func (c *MockClient) Register(version primitive.ProtocolVersion) {
	atomic.CompareAndSwapInt32(&c.registered, 0, int32(version))
}

func (c MockClient) Keyspace() string {
	return c.keyspace
}

func (c MockClient) Local() MockHost {
	return c.server.local
}

func (c *MockClient) Receive(reader io.Reader) error {
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

	if handler, ok := c.server.Handlers[frm.Header.OpCode]; ok {
		c.send(frm.Header, handler(c, frm))
	} else {
		c.send(frm.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}

	return nil
}

func (c MockClient) filterValues(version primitive.ProtocolVersion, stmt *parser.SelectStatement,
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

func (c *MockClient) InterceptQuery(hdr *frame.Header, msg *message.Query) message.Message {
	handled, stmt, err := parser.IsQueryHandled(parser.IdentifierFromString(c.keyspace), msg.Query)

	if handled {
		if err != nil {
			return &message.Invalid{ErrorMessage: err.Error()}
		}

		switch s := stmt.(type) {
		case *parser.SelectStatement:
			if s.Table == "local" {
				vals := c.makeSystemLocalValues(hdr.Version, mockSchemaVersion)
				localColumns := parser.SystemLocalColumns
				if len(c.server.DseVersion) > 0 {
					localColumns = parser.DseSystemLocalColumns
				}
				if columns, err := parser.FilterColumns(s, localColumns); err != nil {
					return &message.Invalid{ErrorMessage: err.Error()}
				} else if row, err := c.filterValues(hdr.Version, s, localColumns, vals, 1); err != nil {
					return &message.Invalid{ErrorMessage: err.Error()}
				} else {
					return &message.RowsResult{
						Metadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
						Data: []message.Row{row},
					}
				}
			} else if s.Table == "peers" {
				peersColumns := parser.SystemPeersColumns
				if len(c.server.DseVersion) > 0 {
					peersColumns = parser.DseSystemPeersColumns
				}
				if columns, err := parser.FilterColumns(s, peersColumns); err != nil {
					return &message.Invalid{ErrorMessage: err.Error()}
				} else {
					var data []message.Row
					peers := c.server.copyPeers()
					for _, peer := range peers {
						vals := c.makeSystemPeerValues(hdr.Version, peer.IP, peer.HostID, mockSchemaVersion)
						if row, err := c.filterValues(hdr.Version, s, peersColumns, vals, len(peers)); err != nil {
							return &message.Invalid{ErrorMessage: err.Error()}
						} else {
							data = append(data, row)
						}
					}
					return &message.RowsResult{
						Metadata: &message.RowsMetadata{
							ColumnCount: int32(len(columns)),
							Columns:     columns,
						},
						Data: data,
					}
				}
			} else {
				return &message.Invalid{ErrorMessage: "Doesn't exist"}
			}
		case *parser.UseStatement:
			c.keyspace = s.Keyspace
			return &message.SetKeyspaceResult{Keyspace: s.Keyspace}
		default:
			return &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"}
		}
	}
	return nil
}

func (c *MockClient) send(hdr *frame.Header, msg message.Message) {
	_ = c.conn.Write(SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *MockClient) makeSystemLocalValues(version primitive.ProtocolVersion, schemaVersion *primitive.UUID) map[string]message.Column {
	ip := net.ParseIP(c.server.local.IP)
	values := c.makeSystemValues(version, ip, c.server.local.HostID, schemaVersion)
	values["key"] = encodeTypeFatal(version, datatype.Varchar, "local")
	values["partitioner"] = encodeTypeFatal(version, datatype.Varchar, "")
	values["cluster_name"] = encodeTypeFatal(version, datatype.Varchar, "cql-proxy")
	values["cql_version"] = encodeTypeFatal(version, datatype.Varchar, "3.4.5")
	values["native_protocol_version"] = encodeTypeFatal(version, datatype.Varchar, version.String())
	return values
}

func (c *MockClient) makeSystemPeerValues(version primitive.ProtocolVersion, address string, hostID, schemaVersion *primitive.UUID) map[string]message.Column {
	ip := net.ParseIP(address)
	values := c.makeSystemValues(version, ip, hostID, schemaVersion)
	values["peer"] = encodeTypeFatal(version, datatype.Inet, ip)
	return values
}

func (c *MockClient) makeSystemValues(version primitive.ProtocolVersion, address net.IP, hostID, schemaVersion *primitive.UUID) map[string]message.Column {
	values := map[string]message.Column{
		"rpc_address":     encodeTypeFatal(version, datatype.Inet, address),
		"data_center":     encodeTypeFatal(version, datatype.Varchar, "dc1"),
		"rack":            encodeTypeFatal(version, datatype.Varchar, "rack1"),
		"tokens":          encodeTypeFatal(version, datatype.NewList(datatype.Varchar), []string{"0"}),
		"release_version": encodeTypeFatal(version, datatype.Varchar, "3.11.10"),
		"host_id":         encodeTypeFatal(version, datatype.Uuid, hostID),
		"schema_version":  encodeTypeFatal(version, datatype.Uuid, schemaVersion),
	}
	if len(c.server.DseVersion) > 0 {
		values["dse_version"] = encodeTypeFatal(version, datatype.Varchar, c.server.DseVersion)
	}
	return values
}

func (c *MockClient) Closing(_ error) {
	c.server.clients.Delete(c)
}

type MockServer struct {
	wg         sync.WaitGroup
	cancel     context.CancelFunc
	clients    sync.Map
	maxVersion primitive.ProtocolVersion
	local      MockHost
	peers      []MockHost
	mu         sync.Mutex
	DseVersion string
	Handlers   map[primitive.OpCode]MockRequestHandler
}

func (s *MockServer) Add(host MockHost) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if host.equal(s.local) {
		return
	}
	for _, peer := range s.peers {
		if host.equal(peer) {
			return
		}
	}
	s.peers = append(s.peers, host)
	s.Event(&message.TopologyChangeEvent{
		ChangeType: primitive.TopologyChangeTypeNewNode,
		Address: &primitive.Inet{
			Addr: net.ParseIP(host.IP),
			Port: int32(host.Port),
		},
	})
}

func (s *MockServer) Remove(host MockHost) {
	if host.equal(s.local) {
		return
	}
	s.mu.Lock()
	s.peers = removeHost(s.peers, host)
	s.mu.Unlock()
	s.Event(&message.TopologyChangeEvent{
		ChangeType: primitive.TopologyChangeTypeRemovedNode,
		Address: &primitive.Inet{
			Addr: net.ParseIP(host.IP),
			Port: int32(host.Port),
		},
	})
}

func (s *MockServer) Shutdown() {
	s.cancel()
	s.wg.Wait()
}

func (s *MockServer) Event(evt message.Event) {
	s.clients.Range(func(key, _ interface{}) bool {
		cl := key.(*MockClient)
		cl.events <- evt
		return true
	})
}

func (s *MockServer) copyPeers() []MockHost {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := make([]MockHost, len(s.peers))
	copy(c, s.peers)
	return c
}

func (s *MockServer) Serve(ctx context.Context, maxVersion primitive.ProtocolVersion, local MockHost, peers []MockHost) error {
	var listener *net.TCPListener
	var err error
	for {
		tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(local.IP, strconv.Itoa(local.Port)))
		if err != nil {
			break
		}

		listener, err = net.ListenTCP("tcp", tcpAddr)
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

	if s.Handlers == nil {
		s.Handlers = DefaultMockRequestHandlers
	}

	ctx, s.cancel = context.WithCancel(ctx)

	s.maxVersion = maxVersion
	s.local = local

	s.peers = make([]MockHost, len(peers))
	copy(s.peers, peers)
	s.peers = removeHost(s.peers, local)

	s.wg.Add(1)

	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				s.wg.Done()
				break
			}
			cl := newMockClient(s)
			cl.conn = NewConn(c, cl)
			go func(cl *MockClient) {
				done := false
				for !done {
					select {
					case event := <-cl.events:
						registered := atomic.LoadInt32(&cl.registered)
						if registered != 0 {
							_ = cl.conn.Write(SenderFunc(func(writer io.Writer) error {
								return codec.EncodeFrame(frame.NewFrame(primitive.ProtocolVersion(registered), -1, event), writer)
							}))
						}
					case <-cl.conn.IsClosed():
						done = true
					}
				}
			}(cl)
			s.clients.Store(cl, struct{}{})
			cl.conn.Start()
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
			s.clients.Range(func(key, _ interface{}) bool {
				cl := key.(*MockClient)
				_ = cl.conn.Close()
				return true
			})
		}
	}()

	return nil
}

type MockCluster struct {
	startHostID *primitive.UUID
	startIP     net.IP
	port        int
	hosts       []MockHost
	servers     map[string]*MockServer
	DseVersion  string
	Handlers    map[primitive.OpCode]MockRequestHandler
}

func NewMockCluster(startIP net.IP, port int) *MockCluster {
	hostID, _ := primitive.ParseUuid("b3bca296-5bb7-411d-b875-67c33fe10000")
	return &MockCluster{
		startHostID: hostID,
		startIP:     startIP,
		port:        port,
		servers:     make(map[string]*MockServer),
	}
}

func (c MockCluster) generate(n int) (host MockHost) {
	host.Port = c.port
	host.HostID = &primitive.UUID{}
	new(big.Int).Add(new(big.Int).SetBytes(c.startHostID[:]), big.NewInt(int64(n))).FillBytes(host.HostID[:])
	ip := make(net.IP, net.IPv6len)
	new(big.Int).Add(new(big.Int).SetBytes(c.startIP), big.NewInt(int64(n))).FillBytes(ip)
	host.IP = ip.String()
	return host
}

func (c *MockCluster) Add(ctx context.Context, n int) error {
	host := c.generate(n)
	for _, h := range c.hosts {
		if host.equal(h) {
			return errors.New("host already added")
		}
	}
	c.hosts = append(c.hosts, host)
	err := c.maybeStart(ctx, host)
	if err != nil {
		return err
	}
	for _, s := range c.servers {
		s.Add(host)
	}
	return nil
}

func (c *MockCluster) maybeStart(ctx context.Context, host MockHost) error {
	key := host.String()
	if _, ok := c.servers[key]; !ok {
		server := &MockServer{DseVersion: c.DseVersion, Handlers: c.Handlers}
		err := server.Serve(ctx, primitive.ProtocolVersion4, host, c.hosts)
		if err != nil {
			return err
		}
		c.servers[key] = server
	}
	return nil
}

func (c *MockCluster) Start(ctx context.Context, n int) error {
	return c.maybeStart(ctx, c.generate(n))
}

func (c *MockCluster) Remove(n int) {
	host := c.generate(n)
	removed := removeHost(c.hosts, host)
	if len(removed) == len(c.hosts) {
		return
	}
	for _, s := range c.servers {
		s.Remove(host)
	}
	c.maybeStop(host)
}

func (c *MockCluster) maybeStop(host MockHost) {
	key := host.String()
	if s, ok := c.servers[key]; ok {
		s.cancel()
		delete(c.servers, key)
	}
}

func (c *MockCluster) Stop(n int) {
	c.maybeStop(c.generate(n))
}

func (c *MockCluster) Shutdown() {
	for _, server := range c.servers {
		server.Shutdown()
	}
}

func encodeTypeFatal(version primitive.ProtocolVersion, dt datatype.DataType, val interface{}) []byte {
	encoded, err := EncodeType(dt, version, val)
	if err != nil {
		panic("unable to encode type")
	}
	return encoded
}

func removeHost(hosts []MockHost, host MockHost) []MockHost {
	for i, h := range hosts {
		if h.equal(host) {
			hosts = append(hosts[:i], hosts[i+1:]...)
			return hosts
		}
	}
	return hosts
}
