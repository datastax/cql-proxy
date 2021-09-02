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
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"cql-proxy/proxycore"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxy_ListenAndServe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const clusterContactPoint = "127.0.0.1:8000"
	const clusterPort = 8000

	const proxyContactPoint = "127.0.0.1:9042"

	cluster := proxycore.NewMockCluster(net.ParseIP("127.0.0.0"), clusterPort)
	defer cluster.Shutdown()

	cluster.Handlers = proxycore.NewMockRequestHandlers(proxycore.MockRequestHandlers{
		primitive.OpCodeQuery: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
				return msg
			} else {
				column, err := proxycore.EncodeType(datatype.Varchar, frm.Header.Version, net.JoinHostPort(cl.Local().IP, strconv.Itoa(cl.Local().Port)))
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

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	err = cluster.Add(ctx, 2)
	require.NoError(t, err)

	err = cluster.Add(ctx, 3)
	require.NoError(t, err)

	proxy := NewProxy(ctx, Config{
		Version:         primitive.ProtocolVersion4,
		Resolver:        proxycore.NewResolverWithDefaultPort([]string{clusterContactPoint}, clusterPort),
		ReconnectPolicy: proxycore.NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
		NumConns:        2,
	})

	err = proxy.Listen(proxyContactPoint)
	defer func(proxy *Proxy) {
		_ = proxy.Shutdown()
	}(proxy)
	require.NoError(t, err)

	go func() {
		_ = proxy.Serve()
	}()

	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpoint(proxyContactPoint), proxycore.ClientConnConfig{})
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion4, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version)

	hosts, err := testQueryHosts(ctx, cl)
	require.NoError(t, err)
	assert.Equal(t, 3, len(hosts))

	cluster.Stop(1)

	removed := waitUntil(10*time.Second, func() bool {
		hosts, err := testQueryHosts(ctx, cl)
		require.NoError(t, err)
		return len(hosts) == 2
	})
	assert.True(t, removed)

	err = cluster.Start(ctx, 1)
	require.NoError(t, err)

	added := waitUntil(10*time.Second, func() bool {
		hosts, err := testQueryHosts(ctx, cl)
		require.NoError(t, err)
		return len(hosts) == 3
	})
	assert.True(t, added)
}

func TestProxy_Unprepared(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numNodes = 3

	const clusterContactPoint = "127.0.0.1:8000"
	const clusterPort = 8000

	const proxyContactPoint = "127.0.0.1:9042"
	const version = primitive.ProtocolVersion4

	preparedId := []byte("abc")

	cluster := proxycore.NewMockCluster(net.ParseIP("127.0.0.0"), clusterPort)
	defer cluster.Shutdown()

	var prepared sync.Map

	cluster.Handlers = proxycore.NewMockRequestHandlers(proxycore.MockRequestHandlers{
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

	for i := 1; i <= numNodes; i++ {
		err := cluster.Add(ctx, i)
		require.NoError(t, err)
	}

	proxy := NewProxy(ctx, Config{
		Version:         version,
		Resolver:        proxycore.NewResolverWithDefaultPort([]string{clusterContactPoint}, clusterPort),
		ReconnectPolicy: proxycore.NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
		NumConns:        2,
	})

	err := proxy.Listen(proxyContactPoint)
	defer func(proxy *Proxy) {
		_ = proxy.Shutdown()
	}(proxy)
	require.NoError(t, err)

	go func() {
		_ = proxy.Serve()
	}()

	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpoint(proxyContactPoint), proxycore.ClientConnConfig{})
	require.NoError(t, err)

	negotiated, err := cl.Handshake(ctx, version, nil)
	require.NoError(t, err)
	assert.Equal(t, version, negotiated)

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

func testQueryHosts(ctx context.Context, cl *proxycore.ClientConn) (map[string]struct{}, error) {
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
