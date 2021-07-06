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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestConnClientAndServer(t *testing.T) {
	ctx := context.Background()

	listener, err := net.Listen("tcp", "127.0.0.1:8123")
	require.NoError(t, err, "failed to listen")

	clientData := randomData(64 * 1024)
	serverData := randomData(64 * 1024)

	serverRecv := &testRecv{
		expected: clientData,
	}
	servClosed := make(chan struct{})

	go func() {
		c, err := listener.Accept()
		require.NoError(t, err, "failed to accept client connection")
		conn := NewConn(c, serverRecv)
		conn.Start()
		err = writeInChunks(conn, serverData, 100)
		require.NoError(t, err, "failed to write bytes to client")
		select {
		case <-conn.IsClosed():
			close(servClosed)
		}
	}()

	clientRecv := &testRecv{
		expected: serverData,
	}
	clientConn, err := Connect(ctx, &defaultEndpoint{"127.0.0.1:8123"}, clientRecv)
	require.NoError(t, err, "failed to connect")

	err = writeInChunks(clientConn, clientData, 100)
	require.NoError(t, err, "failed to write bytes to server")

	timer := time.NewTimer(2 * time.Second)

	select {
	case <-clientConn.IsClosed():
	case <-timer.C:
		require.Fail(t, "timed out waiting for client")
	}

	select {
	case <-servClosed:
	case <-timer.C:
		require.Fail(t, "timed out waiting for server")
	}

	assert.True(t, serverRecv.closed, "server closing method never called")
	assert.True(t, clientRecv.closed, "client closing method never called")
}

func TestConnectFailures(t *testing.T) {
	var tests = []struct {
		endpoint Endpoint
		err      string
	}{
		{endpoint: &defaultEndpoint{"127.0.0.1:8333"}, err: "connection refused"},
		{endpoint: &testEndpoint{addr: "127.0.0.1"}, err: "missing port in address"},
	}
	ctx := context.Background()
	for _, tt := range tests {
		_, err := Connect(ctx, tt.endpoint, &testRecv{})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), tt.err)
		}
	}
}

type testRecv struct {
	expected []byte
	buf      bytes.Buffer
	closed   bool
}

func randomData(n int) []byte {
	data := make([]byte, n)
	for i := 0; i < n; i++ {
		data[i] = 'a' + byte(rand.Intn(26))
	}
	return data
}

func (t *testRecv) Receive(reader io.Reader) error {
	var buf [1024]byte
	//n, err := io.ReadAtLeast(reader, buf[:], 1)
	n, err := reader.Read(buf[:])
	if err != nil {
		return err
	}
	t.buf.Write(buf[:n])
	if bytes.Equal(t.buf.Bytes(), t.expected) {
		return io.EOF
	}
	return nil
}

func (t *testRecv) Closing(_ error) {
	t.closed = true
}

func writeInChunks(conn *Conn, data []byte, n int) (err error) {
	l := len(data)
	remaining := l
	for remaining > 0 {
		w := n
		if remaining < n {
			w = remaining
		}
		o := l - remaining
		err = conn.WriteBytes(data[o : o+w])
		if err != nil {
			return err
		}
		remaining -= w
	}
	return err
}
