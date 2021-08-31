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
	"sync"
	"testing"
	"time"

	"cql-proxy/parser"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectPool(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	p, err := connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
		},
	})
	require.NoError(t, err)

	cl1 := p.leastBusyConn()
	assert.NotNil(t, cl1) // Expect a valid connection

	var wg sync.WaitGroup
	wg.Add(1)

	err = cl1.Send(&testInflightRequest{&wg})
	require.NoError(t, err)

	cl2 := p.leastBusyConn()
	assert.True(t, cl1 != cl2) // cl1 is no longer the least busy

	wg.Wait()
}

func TestConnectPool_NoServer(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	p, err := connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			ReconnectPolicy: NewReconnectPolicyWithDelays(100*time.Millisecond, time.Second),
			NumConns:        2,
			Version:         supported,
		},
	})
	require.NoError(t, err) // Not a critical failure, no error returned

	conn := p.leastBusyConn()
	assert.Nil(t, conn)

	// Start server
	err = server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	reconnected := waitUntil(10*time.Second, func() bool {
		return p.leastBusyConn() != nil
	})
	assert.True(t, reconnected, "expected pool to reconnect after server starts")
}

func TestConnectPool_InvalidAuth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion4
	const username = "username"
	const password = "password"

	server := mockServerWithAuth(username, password)

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			Auth:            NewPasswordAuth("invalid", "invalid"),
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
		},
	})
	if assert.Error(t, err) {
		cqlErr := err.(*CqlError)
		assert.Equal(t, cqlErr, &CqlError{Message: &message.Unauthorized{ErrorMessage: "Invalid credentials"}})
	}
}

func TestConnectPool_AuthExpected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion4
	const username = "username"
	const password = "password"

	server := mockServerWithAuth(username, password)

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			Auth:            nil, // No auth provided
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "authentication required, but no authenticator provided")
	}
}

func TestConnectPool_InvalidProtocolVersion(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2
	const wanted = primitive.ProtocolVersion4

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         wanted,
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "required protocol version is not supported")
	}
}

func TestConnectPool_InvalidKeyspace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion4

	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeQuery: func(cl *MockClient, frm *frame.Frame) message.Message {
				msg := frm.Body.Message.(*message.Query)
				handled, _, stmt := parser.Parse(cl.keyspace, msg.Query)

				if handled {
					switch stmt.(type) {
					case *parser.UseStatement:
						return &message.Invalid{ErrorMessage: "Keyspace doesn't exist"}
					default:
						return &message.ServerError{ErrorMessage: "Proxy attempted to intercept an unhandled query"}
					}
				} else {
					return &message.RowsResult{
						Metadata: &message.RowsMetadata{
							ColumnCount: 0,
						},
						Data: message.RowSet{},
					}
				}
			},
		}),
	}

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			Keyspace:        "keyspace", // Set keyspace
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
		},
	})
	if assert.Error(t, err) {
		cqlErr := err.(*CqlError)
		assert.Equal(t, cqlErr, &CqlError{Message: &message.Invalid{ErrorMessage: "Keyspace doesn't exist"}})
	}
}

func TestConnectPool_InvalidDNS(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "dne:9042"}, // DNS that won't resolve
		SessionConfig: SessionConfig{
			NumConns: 2,
			Version:  supported,
		},
	})
	assert.Error(t, err) // Errors for DNS can vary per system
}

func TestConnectPool_InvalidAddress(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1"}, // Host without a port
		SessionConfig: SessionConfig{
			NumConns: 2,
			Version:  supported,
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing port in address")
	}
}

func TestConnectPool_Timeout(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
			ConnectTimeout:  1 * time.Nanosecond, // set timeout ridiculously low to trigger error
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "dial tcp 127.0.0.1:9042: i/o timeout")
	}
}

func TestConnectPool_HandshakeTimeout(t *testing.T) {
	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeStartup: func(client *MockClient, frm *frame.Frame) message.Message {
				time.Sleep(2 * time.Second)
				return &message.Ready{}
			},
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	_, err = connectPool(ctx, connPoolConfig{
		Endpoint: &defaultEndpoint{addr: "127.0.0.1:9042"},
		SessionConfig: SessionConfig{
			ReconnectPolicy: NewReconnectPolicy(),
			NumConns:        2,
			Version:         supported,
			ConnectTimeout:  1 * time.Second,
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "handshake took longer than 1s to complete")
	}
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
