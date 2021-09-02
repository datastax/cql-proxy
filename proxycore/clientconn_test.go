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
	"bytes"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClientConn_Handshake(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, primitive.ProtocolVersion4, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion4, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version)
}

func TestClientConn_HandshakeNegotiateProtocolVersion(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2
	const starting = primitive.ProtocolVersion4

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, starting, nil)
	require.NoError(t, err)
	assert.Equal(t, supported, version)
}

func TestClientConn_HandshakePasswordAuth(t *testing.T) {
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

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, NewPasswordAuth(username, password))
	require.NoError(t, err)
}

func TestConnectClientWithEvents(t *testing.T) {
	var server MockServer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2
	const starting = primitive.ProtocolVersion4

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	events := make(chan *frame.Frame)
	cl, err := ConnectClientWithEvents(ctx, &defaultEndpoint{"127.0.0.1:9042"}, EventHandlerFunc(func(frm *frame.Frame) {
		events <- frm
	}))
	require.NoError(t, err)

	wait := func() *frame.Frame {
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-timer.C:
			require.Fail(t, "timed out waiting for event")
		case event := <-events:
			return event
		}
		require.Fail(t, "event expected")
		return nil
	}

	version, err := cl.Handshake(ctx, starting, nil)
	require.NoError(t, err)
	assert.Equal(t, supported, version)

	status := &message.StatusChangeEvent{ChangeType: primitive.StatusChangeTypeUp, Address: &primitive.Inet{
		Addr: net.ParseIP("192.168.1.42"),
		Port: 9042,
	}}
	server.Event(status)
	received := wait()
	assert.Equal(t, status, received.Body.Message)
}

func TestClientConn_HandshakePasswordInvalidAuth(t *testing.T) {
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

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, NewPasswordAuth("invalid", "invalid"))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Invalid credentials")
	}
}

func TestClientConn_HandshakeAuthRequireButNotProvided(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion2
	const starting = primitive.ProtocolVersion4

	server := mockServerWithAuth("username", "password")

	err := server.Serve(ctx, supported, MockHost{
		IP:   "127.0.0.1",
		Port: 9042,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, starting, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "authentication required, but no authenticator provided")
	}
}

func TestClientConn_Query(t *testing.T) {
	var server MockServer

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	rs, err := cl.Query(ctx, supported, &message.Query{
		Query: "SELECT * FROM system.local",
	})
	require.NoError(t, err)
	require.Equal(t, rs.RowCount(), 1)

	valueByKey := func(key string) interface{} {
		row := rs.Row(0)
		val, err := row.ByName(key)
		require.NoError(t, err)
		return val
	}

	assert.Equal(t, "local", valueByKey("key"))
	assert.Equal(t, net.ParseIP("127.0.0.1").To4(), valueByKey("rpc_address"))
	assert.Equal(t, *mockHostID, valueByKey("host_id"))
}

func TestClientConn_SetKeyspace(t *testing.T) {
	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeQuery: func(cl *MockClient, frm *frame.Frame) message.Message {
				if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
					return msg
				} else {
					return &message.Invalid{ErrorMessage: "Doesn't exist"}
				}
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	_, err = cl.Query(ctx, supported, &message.Query{
		Query: "SELECT * FROM local",
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Doesn't exist")
	}

	err = cl.SetKeyspace(ctx, supported, "system")
	require.NoError(t, err)

	rs, err := cl.Query(ctx, supported, &message.Query{
		Query: "SELECT * FROM local",
	})
	require.NoError(t, err)
	require.Equal(t, rs.RowCount(), 1)
}

func TestClientConn_Inflight(t *testing.T) {
	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeQuery: func(cl *MockClient, frm *frame.Frame) message.Message {
				time.Sleep(100 * time.Millisecond) // Give time to make sure we're able to count inflight requests
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
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	const expected = 10

	var wg sync.WaitGroup
	wg.Add(expected)

	for i := 0; i < 10; i++ {
		err := cl.Send(&testInflightRequest{&wg})
		require.NoError(t, err)
	}

	assert.Equal(t, int32(expected), cl.Inflight()) // Verify async inflight requests
	wg.Wait()
	assert.Equal(t, int32(0), cl.Inflight()) // Should be 0 after they complete
}

type testInflightRequest struct {
	wg *sync.WaitGroup
}

func (t testInflightRequest) Frame() interface{} {
	return frame.NewFrame(primitive.ProtocolVersion4, -1, &message.Query{
		Query: "SELECT * FROM system.local",
	})
}

func (t testInflightRequest) OnClose(_ error) {
}

func (t testInflightRequest) OnResult(_ *frame.RawFrame) {
	t.wg.Done()
}

func mockServerWithAuth(username, password string) *MockServer {
	return &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeStartup: func(client *MockClient, frm *frame.Frame) message.Message {
				return &message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"}
			},
			primitive.OpCodeAuthResponse: func(client *MockClient, frm *frame.Frame) message.Message {
				response := frm.Body.Message.(*message.AuthResponse)
				source := bytes.NewBuffer(append(response.Token, 0))
				if _, err := source.ReadBytes(0); err != nil {
					return &message.AuthenticationError{ErrorMessage: "Invalid token (authId)"}
				} else if u, err := source.ReadString(0); err != nil {
					return &message.AuthenticationError{ErrorMessage: "Invalid token (username)"}
				} else if p, err := source.ReadString(0); err != nil {
					return &message.AuthenticationError{ErrorMessage: "Invalid token (password)"}
				} else if u[:len(u)-1] == username && p[:len(p)-1] == password {
					return &message.AuthSuccess{}
				} else {
					return &message.Unauthorized{ErrorMessage: "Invalid credentials"}
				}
			},
		}),
	}
}

const (
	connectTimeout    = 100 * time.Millisecond
	heartbeatInterval = 500 * time.Millisecond
	idleTimeout       = 1000 * time.Millisecond
)

func TestClientConn_Heartbeats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	heartbeatCh := make(chan bool, 1)

	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeOptions: func(cl *MockClient, frm *frame.Frame) message.Message {
				heartbeatCh <- true
				return &message.Supported{}
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	go cl.Heartbeats(connectTimeout, supported, heartbeatInterval, idleTimeout, logger)
	select {
	case <-heartbeatCh:
		_ = cl.Close()
	case <-time.After(idleTimeout * 2):
		assert.Fail(t, "expected heartbeat")
	}
}

func TestClientConn_HeartbeatsError(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeOptions: func(cl *MockClient, frm *frame.Frame) message.Message {
				return &message.ServerError{}
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	go cl.Heartbeats(connectTimeout, supported, heartbeatInterval, idleTimeout, logger)
	closed := waitUntil(2*idleTimeout, func() bool {
		select {
		case <-cl.IsClosed():
			return true
		default:
			return false
		}
	})
	assert.True(t, closed, "expected the connection to be closed")
}

func TestClientConn_HeartbeatsTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeOptions: func(cl *MockClient, frm *frame.Frame) message.Message {
				time.Sleep(heartbeatInterval)
				return &message.Supported{}
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	go cl.Heartbeats(connectTimeout, supported, heartbeatInterval, idleTimeout, logger)
	closed := waitUntil(2*idleTimeout, func() bool {
		select {
		case <-cl.IsClosed():
			return true
		default:
			return false
		}
	})
	assert.True(t, closed, "expected the connection to be closed")

}

func TestClientConn_HeartbeatsUnexpectedMessage(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := &MockServer{
		Handlers: NewMockRequestHandlers(MockRequestHandlers{
			primitive.OpCodeOptions: func(cl *MockClient, frm *frame.Frame) message.Message {
				return &message.Startup{}
			},
		}),
	}

	const supported = primitive.ProtocolVersion4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Serve(ctx, supported, MockHost{
		IP:     "127.0.0.1",
		Port:   9042,
		HostID: mockHostID,
	}, nil)
	require.NoError(t, err)

	cl, err := ConnectClient(ctx, &defaultEndpoint{"127.0.0.1:9042"})
	require.NoError(t, err)

	_, err = cl.Handshake(ctx, supported, nil)
	require.NoError(t, err)

	go cl.Heartbeats(connectTimeout, supported, heartbeatInterval, idleTimeout, logger)
	closed := waitUntil(2*idleTimeout, func() bool {
		select {
		case <-cl.IsClosed():
			return true
		default:
			return false
		}
	})
	assert.True(t, closed, "expected the connection to be closed")

}
