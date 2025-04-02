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
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/datastax/cql-proxy/codecs"
	"github.com/datastax/cql-proxy/proxycore"
	"github.com/gocql/gocql"
	"github.com/pierrec/lz4/v4"
	"go.uber.org/zap"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAddr      = "127.0.0.1"
	testStartAddr = "127.0.0.0"
	sizeOfUint32  = 4
)

func init() {
	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
}

func generateTestPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		log.Panicf("failed to resolve for local port: %v", err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Panicf("failed to listen for local port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func generateTestAddr(baseAddress string, n int) string {
	ip := make(net.IP, net.IPv6len)
	new(big.Int).Add(new(big.Int).SetBytes(net.ParseIP(baseAddress)), big.NewInt(int64(n))).FillBytes(ip)
	return ip.String()
}

func generateTestAddrs(host string) (clusterPort int, clusterAddr, proxyAddr, httpAddr string) {
	clusterPort = generateTestPort()
	clusterAddr = net.JoinHostPort(host, strconv.Itoa(clusterPort))
	proxyPort := generateTestPort()
	proxyAddr = net.JoinHostPort(host, strconv.Itoa(proxyPort))
	httpPort := generateTestPort()
	httpAddr = net.JoinHostPort(host, strconv.Itoa(httpPort))
	return clusterPort, clusterAddr, proxyAddr, httpAddr
}

func TestProxy_ListenAndServe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tester, proxyContactPoint, err := setupProxyTest(ctx, 3, proxycore.MockRequestHandlers{
		primitive.OpCodeQuery: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
				return msg
			} else {
				column, err := codecs.EncodeType(datatype.Varchar, frm.Header.Version, net.JoinHostPort(cl.Local().IP, strconv.Itoa(cl.Local().Port)))
				if err != nil {
					return &message.ServerError{ErrorMessage: "Unable to encode type"}
				}
				return &message.RowsResult{
					Metadata: &message.RowsMetadata{
						Columns: []*message.ColumnMetadata{
							{
								Keyspace: "test",
								Table:    "test",
								Name:     "host",
								Type:     datatype.Varchar,
							},
						},
						ColumnCount: 1,
					},
					Data: message.RowSet{{
						column,
					}},
				}
			}
		},
	})
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	hosts, err := queryTestHosts(ctx, cl)
	require.NoError(t, err)
	assert.Equal(t, 3, len(hosts))

	tester.cluster.Stop(1)

	removed := waitUntil(10*time.Second, func() bool {
		hosts, err := queryTestHosts(ctx, cl)
		require.NoError(t, err)
		return len(hosts) == 2
	})
	assert.True(t, removed)

	err = tester.cluster.Start(ctx, 1)
	require.NoError(t, err)

	added := waitUntil(10*time.Second, func() bool {
		hosts, err := queryTestHosts(ctx, cl)
		require.NoError(t, err)
		return len(hosts) == 3
	})
	assert.True(t, added)
}

func TestProxy_Unprepared(t *testing.T) {
	const numNodes = 3
	const version = primitive.ProtocolVersion4

	preparedId := []byte("abc")
	var prepared sync.Map

	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, numNodes, proxycore.MockRequestHandlers{
		primitive.OpCodePrepare: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			prepared.Store(cl.Local().IP, true)
			return &message.PreparedResult{
				PreparedQueryId: preparedId,
			}
		},
		primitive.OpCodeExecute: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			if _, ok := prepared.Load(cl.Local().IP); ok {
				return &message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: 0,
					},
					Data: message.RowSet{},
				}
			} else {
				ex := frm.Body.Message.(*message.Execute)
				assert.Equal(t, preparedId, ex.QueryId)
				return &message.Unprepared{Id: ex.QueryId}
			}
		},
	})
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	// Only prepare on a single node
	resp, err := cl.SendAndReceive(ctx, frame.NewFrame(version, 0, &message.Prepare{Query: "SELECT * FROM test.test"}))
	require.NoError(t, err)
	assert.Equal(t, primitive.OpCodeResult, resp.Header.OpCode)
	_, ok := resp.Body.Message.(*message.PreparedResult)
	assert.True(t, ok, "expected prepared result")

	for i := 0; i < numNodes; i++ {
		resp, err = cl.SendAndReceive(ctx, frame.NewFrame(version, 0, &message.Execute{QueryId: preparedId}))
		require.NoError(t, err)
		assert.Equal(t, primitive.OpCodeResult, resp.Header.OpCode)
		_, ok = resp.Body.Message.(*message.RowsResult)
		assert.True(t, ok, "expected rows result")
	}

	// Count the number of unique nodes that were prepared
	count := 0
	prepared.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, numNodes, count)
}

func TestProxy_UseKeyspace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, 1, nil)
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	testKeyspaces := []string{"system", "\"system\""}
	for _, testKeyspace := range testKeyspaces {

		resp, err := cl.SendAndReceive(ctx, frame.NewFrame(primitive.ProtocolVersion4, 0, &message.Query{Query: "USE " + testKeyspace}))
		require.NoError(t, err)

		assert.Equal(t, primitive.OpCodeResult, resp.Header.OpCode)
		res, ok := resp.Body.Message.(*message.SetKeyspaceResult)
		require.True(t, ok, "expected set keyspace result")
		assert.Equal(t, "system", res.Keyspace)
	}
}

func TestProxy_UseKeyspace_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, 1, proxycore.MockRequestHandlers{
		primitive.OpCodeQuery: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			qry := frm.Body.Message.(*message.Query)
			if qry.Query == "USE non_existing" {
				return &message.ServerError{
					ErrorMessage: "Keyspace 'non_existing' does not exist",
				}
			}
			return cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query))
		}})
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	resp, err := cl.SendAndReceive(ctx, frame.NewFrame(primitive.ProtocolVersion4, 0, &message.Query{Query: "USE non_existing"}))
	require.NoError(t, err)

	assert.Equal(t, primitive.OpCodeError, resp.Header.OpCode)
	res, ok := resp.Body.Message.(*message.ServerError)
	require.True(t, ok)
	// make sure that CQL Proxy returns the same error of 'USE keyspace' command
	// as backend C* cluster has and does not wrap it inside a custom one
	assert.Equal(t, "Keyspace 'non_existing' does not exist", res.ErrorMessage)
}

func TestProxy_NegotiateProtocolV5(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, 1, nil)
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpoint(proxyContactPoint), proxycore.ClientConnConfig{})
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion5, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version) // Expected to be negotiated to v4
}

func TestProxy_DseVersion(t *testing.T) {
	const dseVersion = "6.8.3"
	const protocol = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fake a peer so that "SELECT ... system.peers" returns at least one row
	tester, proxyContactPoint, err := setupProxyTestWithConfig(ctx, 1, &proxyTestConfig{
		dseVersion: dseVersion,
		rpcAddr:    "127.0.0.1",
		peers: []PeerConfig{{
			RPCAddr: "127.0.0.2",
		}}})
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	checkDseVersion := func(resp *frame.Frame, err error) {
		require.NoError(t, err)
		assert.Equal(t, primitive.OpCodeResult, resp.Header.OpCode)
		rows, ok := resp.Body.Message.(*message.RowsResult)
		assert.True(t, ok, "expected rows result")

		rs := proxycore.NewResultSet(rows, protocol)
		require.GreaterOrEqual(t, rs.RowCount(), 1)
		actualDseVersion, err := rs.Row(0).StringByName("dse_version")
		require.NoError(t, err)
		assert.Equal(t, dseVersion, actualDseVersion)
	}

	checkDseVersion(cl.SendAndReceive(ctx, frame.NewFrame(protocol, 0, &message.Query{Query: "SELECT dse_version FROM system.local"})))
	checkDseVersion(cl.SendAndReceive(ctx, frame.NewFrame(primitive.ProtocolVersion4, 0, &message.Query{Query: "SELECT dse_version FROM system.peers"})))
}

func TestProxy_UnsupportedCompressionType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, 1, nil)
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	// Send a STARTUP message with an unsupported compression type
	res, err := cl.SendAndReceive(ctx,
		frame.NewFrame(primitive.ProtocolVersion4, 0,
			&message.Startup{Options: map[string]string{"COMPRESSION": "unsupported"}}),
	)
	require.NoError(t, err)
	require.IsType(t, &message.ProtocolError{}, res.Body.Message)

	assert.Contains(t, res.Body.Message.(*message.ProtocolError).ErrorMessage,
		"Unsupported compression type: unsupported (supported compression types: lz4, snappy)")
}

func TestProxy_Compression(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tester, proxyContactPoint, err := setupProxyTest(ctx, 1, nil)
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err)
	tests := []struct {
		compressor gocql.Compressor
	}{
		{
			compressor: gocql.SnappyCompressor{},
		},
		{
			compressor: lz4Compressor{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.compressor.Name(), func(t *testing.T) {
			config := gocql.NewCluster(proxyContactPoint)
			config.Authenticator = &gocql.PasswordAuthenticator{Username: "cassandra", Password: "cassandra"}
			config.Compressor = tt.compressor

			var sess *gocql.Session
			require.True(t, waitUntil(5*time.Second, func() bool {
				var err error
				sess, err = config.CreateSession()
				return err == nil && sess != nil
			}), "failed to create session")
			defer sess.Close()

			assert.True(t, waitUntil(5*time.Second, func() bool {
				var err error
				err = sess.Query("select * from test").Iter().Close()
				return err == nil
			}), "failed to execute query")
		})
	}
}

func queryTestHosts(ctx context.Context, cl *proxycore.ClientConn) (map[string]struct{}, error) {
	hosts := make(map[string]struct{})
	for i := 0; i < 3; i++ {
		rs, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{Query: "SELECT * FROM test.test"})
		if err != nil {
			return nil, err
		}
		if rs.RowCount() < 1 {
			return nil, errors.New("invalid row count")
		}
		val, err := rs.Row(0).ByName("host")
		if err != nil {
			return nil, err
		}
		hosts[val.(string)] = struct{}{}
	}
	return hosts, nil
}

type proxyTester struct {
	cluster *proxycore.MockCluster
	proxy   *Proxy
	wg      sync.WaitGroup
}

func (w *proxyTester) shutdown() {
	w.cluster.Shutdown()
	_ = w.proxy.Close()
	w.wg.Wait()
}

func setupProxyTest(ctx context.Context, numNodes int, handlers proxycore.MockRequestHandlers) (tester *proxyTester, proxyContactPoint string, err error) {
	return setupProxyTestWithConfig(ctx, numNodes, &proxyTestConfig{handlers: handlers})
}

type proxyTestConfig struct {
	handlers        proxycore.MockRequestHandlers
	dseVersion      string
	rpcAddr         string
	peers           []PeerConfig
	idempotentGraph bool
}

func setupProxyTestWithConfig(ctx context.Context, numNodes int, cfg *proxyTestConfig) (tester *proxyTester, proxyContactPoint string, err error) {
	tester = &proxyTester{
		wg: sync.WaitGroup{},
	}

	clusterPort, clusterAddr, proxyAddr, _ := generateTestAddrs(testAddr)

	tester.cluster = proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)
	tester.cluster.DseVersion = cfg.dseVersion

	if cfg == nil {
		cfg = &proxyTestConfig{}
	}

	if cfg.handlers != nil {
		tester.cluster.Handlers = proxycore.NewMockRequestHandlers(cfg.handlers)
	}

	for i := 1; i <= numNodes; i++ {
		err = tester.cluster.Add(ctx, i)
		if err != nil {
			return tester, proxyAddr, err
		}
	}
	tester.proxy = NewProxy(ctx, Config{
		Version:           primitive.ProtocolVersion4,
		Resolver:          proxycore.NewResolverWithDefaultPort([]string{clusterAddr}, clusterPort),
		ReconnectPolicy:   proxycore.NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
		NumConns:          2,
		HeartBeatInterval: 30 * time.Second,
		ConnectTimeout:    10 * time.Second,
		IdleTimeout:       60 * time.Second,
		RPCAddr:           cfg.rpcAddr,
		Peers:             cfg.peers,
		IdempotentGraph:   cfg.idempotentGraph,
		Logger:            zap.L(),
	})

	err = tester.proxy.Connect()
	if err != nil {
		return tester, proxyAddr, err
	}

	l, err := resolveAndListen(proxyAddr, "", "")
	if err != nil {
		return tester, proxyAddr, err
	}

	tester.wg.Add(1)

	go func() {
		_ = tester.proxy.Serve(l)
		tester.wg.Done()
	}()

	return tester, proxyAddr, nil
}

type lz4Compressor struct{}

func (l lz4Compressor) Name() string {
	return "lz4"
}

func (l lz4Compressor) Encode(data []byte) ([]byte, error) {
	maxCompressedSize := lz4.CompressBlockBound(len(data)) + sizeOfUint32
	compressed := make([]byte, maxCompressedSize)

	binary.BigEndian.PutUint32(compressed, uint32(len(data)))

	var compressor lz4.Compressor
	n, err := compressor.CompressBlock(data, compressed[4:])
	if err != nil {
		return nil, err
	}

	return compressed[:n+sizeOfUint32], nil
}

func (l lz4Compressor) Decode(data []byte) ([]byte, error) {
	if len(data) < sizeOfUint32 {
		return nil, fmt.Errorf("unable to read compressed data length; expected at least %d bytes, got %d",
			sizeOfUint32, len(data))
	}
	decompressedLen := binary.BigEndian.Uint32(data)
	decompressed := make([]byte, decompressedLen)
	_, err := lz4.UncompressBlock(data[sizeOfUint32:], decompressed)
	return decompressed, err
}

func connectTestClient(t *testing.T, ctx context.Context, proxyContactPoint string) *proxycore.ClientConn {
	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpoint(proxyContactPoint), proxycore.ClientConnConfig{})
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion4, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version)

	return cl
}

func waitUntil(d time.Duration, check func() bool) bool {
	iterations := int(d / (100 * time.Millisecond))
	for i := 0; i < iterations; i++ {
		if check() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
