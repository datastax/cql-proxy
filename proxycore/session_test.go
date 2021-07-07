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
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"sync"
	"testing"
	"time"
)

func TestConnectSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion4

	c := NewMockCluster(net.ParseIP("127.0.0.0"))

	err := c.Add(ctx, 1)
	require.NoError(t, err)

	err = c.Add(ctx, 2)
	require.NoError(t, err)

	err = c.Add(ctx, 3)
	require.NoError(t, err)

	cluster, err := ConnectCluster(ctx, ClusterConfig{
		RefreshWindow:   100 * time.Millisecond,
		Version:         supported,
		Resolver:        NewResolver("127.0.0.1:9042"),
		ReconnectPolicy: NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
	})
	require.NoError(t, err)

	session, err := ConnectSession(ctx, cluster, SessionConfig{
		ReconnectPolicy: NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
		NumConns:        2,
		Version:         supported,
	})
	require.NoError(t, err)

	newHost := func(addr string) *Host {
		return &Host{endpoint: &defaultEndpoint{addr: addr}}
	}

	var wg sync.WaitGroup

	wg.Add(3)

	err = session.Send(newHost("127.0.0.1:9042"), &testSessionRequest{t: t, rpcAddr: "127.0.0.1", wg: &wg})
	require.NoError(t, err)

	err = session.Send(newHost("127.0.0.2:9042"), &testSessionRequest{t: t, rpcAddr: "127.0.0.2", wg: &wg})
	require.NoError(t, err)

	err = session.Send(newHost("127.0.0.3:9042"), &testSessionRequest{t: t, rpcAddr: "127.0.0.3", wg: &wg})
	require.NoError(t, err)

	wg.Wait()

	err = c.Add(ctx, 4)
	require.NoError(t, err)

	available := waitUntil(10*time.Second, func() bool {
		return session.leastBusyConn(newHost("127.0.0.4:9042")) != nil
	})
	require.True(t, available)

	wg.Add(1)

	err = session.Send(newHost("127.0.0.4:9042"), &testSessionRequest{t: t, rpcAddr: "127.0.0.4", wg: &wg})
	require.NoError(t, err)

	wg.Wait()

	c.Remove(4)

	removed := waitUntil(10*time.Second, func() bool {
		return session.leastBusyConn(newHost("127.0.0.4:9042")) == nil
	})
	require.True(t, removed)
}

type testSessionRequest struct {
	t       *testing.T
	version primitive.ProtocolVersion
	rpcAddr string
	wg      *sync.WaitGroup
}

func (r testSessionRequest) Frame() interface{} {
	return frame.NewFrame(primitive.ProtocolVersion4, -1, &message.Query{
		Query: "SELECT * FROM system.local",
	})
}

func (r testSessionRequest) OnClose(_ error) {
	require.Fail(r.t, "connection unexpectedly closed")
}

func (r testSessionRequest) OnResult(raw *frame.RawFrame) {
	frm, err := codec.ConvertFromRawFrame(raw)
	require.NoError(r.t, err)

	switch msg := frm.Body.Message.(type) {
	case *message.RowsResult:
		rs := NewResultSet(msg, r.version)
		rpcAddr, err := rs.Row(0).ByName("rpc_address")
		require.NoError(r.t, err)
		assert.Equal(r.t, rpcAddr.(net.IP).String(), r.rpcAddr)
	default:
		require.Fail(r.t, "invalid message body")
	}

	r.wg.Done()
}
