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

package astra

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/datastax/cql-proxy/codecs"
	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sniProxyAddr = "localhost:8080"

var contactPoints = []string{
	"a2e24181-d732-402a-ab06-894a8b2f6094",
	"ce00ba58-a377-4022-ba09-00394ee66cfb",
	"9e339fe3-2bf2-45ce-a660-76951f39a8e8",
}

func TestMain(m *testing.M) {
	serv, err := runTestMetaSvcAsync(sniProxyAddr, contactPoints)
	if err != nil {
		panic(err)
	}
	r := m.Run()
	_ = serv.Close()
	os.Exit(r)
}

func TestAstraResolver_Resolve(t *testing.T) {
	resolver := createResolver(t)
	endpoints, err := resolver.Resolve(context.Background())
	require.NoError(t, err)

	for _, endpoint := range endpoints {
		assert.False(t, endpoint.IsResolved())
		assert.Equal(t, sniProxyAddr, endpoint.Addr())
		assert.Contains(t, contactPoints, endpoint.TLSConfig().ServerName)
	}
}

func TestAstraResolver_NewEndpoint(t *testing.T) {
	resolver := createResolver(t)
	_, err := resolver.Resolve(context.Background())
	require.NoError(t, err)

	const hostId = "a2e24181-d732-402a-ab06-894a8b2f6094"

	rs := proxycore.NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "host_id",
					Index:    0,
					Type:     datatype.Uuid,
				},
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "data_center",
					Index:    1,
					Type:     datatype.Varchar,
				},
			},
		},
		Data: message.RowSet{
			message.Row{makeUUID(hostId), makeVarchar("us-east1")},
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.NotNil(t, endpoint)
	assert.Nil(t, err)
	assert.Contains(t, endpoint.Key(), hostId)
}

func TestAstraResolver_NewEndpoint_Ignored(t *testing.T) {
	resolver := createResolver(t)
	_, err := resolver.Resolve(context.Background())
	require.NoError(t, err)

	const hostId = "a2e24181-d732-402a-ab06-894a8b2f6094"

	rs := proxycore.NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "host_id",
					Index:    0,
					Type:     datatype.Uuid,
				},
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "data_center",
					Index:    1,
					Type:     datatype.Varchar,
				},
			},
		},
		Data: message.RowSet{
			message.Row{makeUUID(hostId), makeVarchar("ignored")},
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.Nil(t, endpoint)
	assert.ErrorIs(t, err, proxycore.IgnoreEndpoint)
}

func TestAstraResolver_NewEndpointInvalidHostID(t *testing.T) {
	resolver := createResolver(t)
	_, err := resolver.Resolve(context.Background())
	require.NoError(t, err)

	rs := proxycore.NewResultSet(&message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: "system",
					Table:    "peers",
					Name:     "host_id",
					Index:    0,
					Type:     datatype.Uuid,
				},
			},
		},
		Data: message.RowSet{
			message.Row{nil}, // Null value
		},
	}, primitive.ProtocolVersion4)

	endpoint, err := resolver.NewEndpoint(rs.Row(0))
	assert.Nil(t, endpoint)
	assert.Error(t, err, "ignoring host because its `host_id` is not set or is invalid")
}

func TestAstraResolver_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1) // Very short timeout
	defer cancel()

	resolver := createResolver(t)
	_, err := resolver.Resolve(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded) // Expect a timeout
}

func createResolver(t *testing.T) proxycore.EndpointResolver {
	path, err := writeBundle("127.0.0.1", 8080)
	require.NoError(t, err)

	bundle, err := LoadBundleZipFromPath(path)
	require.NoError(t, err)

	return NewResolver(bundle, 10*time.Second)
}

func runTestMetaSvcAsync(sniProxyAddr string, contactPoints []string) (*http.Server, error) {
	host, _, err := net.SplitHostPort(sniProxyAddr)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := createServerTLSConfig(host)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", sniProxyAddr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metadata", func(writer http.ResponseWriter, request *http.Request) {
		res, err := json.Marshal(astraMetadata{
			Version: 1,
			Region:  "us-east1",
			ContactInfo: contactInfo{
				SniProxyAddress: sniProxyAddr,
				ContactPoints:   contactPoints,
			},
		})
		if err != nil {
			writer.WriteHeader(500)
		} else {
			_, _ = writer.Write(res)
		}
	})

	serv := &http.Server{
		Addr:      sniProxyAddr,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	go func() {
		_ = serv.ServeTLS(listener, "", "")
	}()

	return serv, nil
}

func createServerTLSConfig(dnsName string) (*tls.Config, error) {
	rootCAs, err := createCertPool()
	if err != nil {
		return nil, err
	}

	if !rootCAs.AppendCertsFromPEM(testCAPEM) {
		return nil, errors.New("unable to add cert to CA pool")
	}

	cert, err := tls.X509KeyPair(testCertPEM, testKeyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      rootCAs,
		ClientCAs:    rootCAs,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}

func makeUUID(uuid string) []byte {
	parsedUuid, _ := primitive.ParseUuid(uuid)
	bytes, _ := codecs.EncodeType(datatype.Uuid, primitive.ProtocolVersion4, parsedUuid)
	return bytes
}

func makeVarchar(s string) []byte {
	bytes, _ := codecs.EncodeType(datatype.Varchar, primitive.ProtocolVersion4, s)
	return bytes
}
