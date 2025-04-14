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
	"crypto/tls"
	"net"
	"testing"

	"github.com/datastax/cql-proxy/codecs"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupEndpoint(t *testing.T) {
	addr, err := LookupEndpoint(&testEndpoint{addr: "localhost:9042"})
	require.NoError(t, err, "unable to lookup endpoint")
	assert.True(t, addr == "127.0.0.1:9042" || addr == "[::1]:9042")

	addr, err = LookupEndpoint(&testEndpoint{addr: "127.0.0.1:9042", isResolved: true})
	require.NoError(t, err, "unable to lookup endpoint")
	assert.Equal(t, "127.0.0.1:9042", addr)
}

func TestLookupEndpoint_Invalid(t *testing.T) {
	var tests = []struct {
		addr string
		err  string
	}{
		{"localhost", "missing port in address"},
		{"test:1234", ""}, // Errors for DNS can vary per system
	}

	for _, tt := range tests {
		_, err := LookupEndpoint(&testEndpoint{addr: tt.addr})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), tt.err)
		}
	}
}

func TestEndpoint_NewEndpoint(t *testing.T) {
	resolver := NewResolver("127.0.0.1")

	const rpcAddr = "127.0.0.2"

	rpcAddrBytes, _ := codecs.EncodeType(datatype.Inet, primitive.ProtocolVersion4, net.ParseIP(rpcAddr))

	rs := NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "rpc_address",
					Index:    0,
					Type:     datatype.Inet,
				},
			},
		},
		Data: message.RowSet{
			message.Row{rpcAddrBytes},
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.NotNil(t, endpoint)
	assert.Nil(t, err)
	assert.Contains(t, endpoint.Key(), rpcAddr)
}

func TestEndpoint_NewEndpointUnknownRPCAddress(t *testing.T) {
	resolver := NewResolver("127.0.0.1")

	const rpcAddr = "0.0.0.0"
	rpcAddrBytes, _ := codecs.EncodeType(datatype.Inet, primitive.ProtocolVersion4, net.ParseIP(rpcAddr))

	const peer = "127.0.0.2"
	peerBytes, _ := codecs.EncodeType(datatype.Inet, primitive.ProtocolVersion4, net.ParseIP(peer))

	rs := NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "peer",
					Index:    0,
					Type:     datatype.Inet,
				},
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "rpc_address",
					Index:    1,
					Type:     datatype.Inet,
				},
			},
		},
		Data: message.RowSet{
			message.Row{peerBytes, rpcAddrBytes},
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.NotNil(t, endpoint)
	assert.Nil(t, err)
	assert.Contains(t, endpoint.Key(), peer)
}

func TestEndpoint_NewEndpointInvalidRPCAddress(t *testing.T) {
	resolver := NewResolver("127.0.0.1")

	const peer = "127.0.0.2"
	peerBytes, _ := codecs.EncodeType(datatype.Inet, primitive.ProtocolVersion4, net.ParseIP(peer))

	rs := NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "peer",
					Index:    0,
					Type:     datatype.Inet,
				},
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "rpc_address",
					Index:    1,
					Type:     datatype.Inet,
				},
			},
		},
		Data: message.RowSet{
			message.Row{peerBytes, nil}, // Null rpc_address
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.Nil(t, endpoint)
	assert.Error(t, err, "ignoring host because its `rpc_address` is not set or is invalid")
}

func TestEndpoint_NewEndpointInvalidPeer(t *testing.T) {
	resolver := NewResolver("127.0.0.1")

	const rpcAddr = "0.0.0.0"
	rpcAddrBytes, _ := codecs.EncodeType(datatype.Inet, primitive.ProtocolVersion4, net.ParseIP(rpcAddr))

	rs := NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "peer",
					Index:    0,
					Type:     datatype.Inet,
				},
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "rpc_address",
					Index:    1,
					Type:     datatype.Inet,
				},
			},
		},
		Data: message.RowSet{
			message.Row{nil, rpcAddrBytes}, // Null peer
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.Nil(t, endpoint)
	assert.Error(t, err, "ignoring host because its `peer` is not set or is invalid")
}

type testEndpoint struct {
	addr       string
	isResolved bool
}

func (t testEndpoint) String() string {
	return t.addr
}

func (t testEndpoint) Addr() string {
	return t.addr
}

func (t testEndpoint) IsResolved() bool {
	return t.isResolved
}

func (t testEndpoint) TLSConfig() *tls.Config {
	return nil
}

func (t testEndpoint) Key() string {
	return t.addr
}
