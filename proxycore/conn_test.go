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
	"io"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	ctx := context.Background()

	listener, err := net.Listen("tcp", "127.0.0.1:8123")
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(listener)
	require.NoError(t, err, "failed to listen")

	clientData := randomData(64 * 1024)
	serverData := randomData(64 * 1024)

	serverRecv := newTestRecv(clientData)
	servClosed := make(chan struct{})

	go func() {
		c, err := listener.Accept()
		require.NoError(t, err, "failed to accept client connection")
		conn := NewConn(c, serverRecv)
		conn.Start()
		err = conn.WriteBytes(serverData)
		require.NoError(t, err, "failed to write bytes to client")
		select {
		case <-conn.IsClosed():
			close(servClosed)
		}
	}()

	clientRecv := newTestRecv(serverData)
	clientConn, err := Connect(ctx, NewEndpoint("127.0.0.1:8123"), clientRecv)
	require.NoError(t, err, "failed to connect")

	err = clientConn.WriteBytes(clientData)
	require.NoError(t, err, "failed to write bytes to server")

	timer := time.NewTimer(2 * time.Second)

	wait := func(waitFor chan struct{}, msg string) {
		select {
		case <-waitFor:
		case <-timer.C:
			require.Fail(t, msg)
		}
	}

	wait(clientRecv.received, "timed out waiting to receive data from the server")
	wait(serverRecv.received, "timed out waiting to receive data from the client")

	_ = clientConn.Close()

	wait(clientConn.IsClosed(), "timed out waiting for client to close")
	wait(servClosed, "timed out waiting for server to close")

	wait(clientRecv.closing, "client closing method never called")
	wait(serverRecv.closing, "server closing method never called")
}

func TestConnect_Failures(t *testing.T) {
	var tests = []struct {
		endpoint Endpoint
		err      string
	}{
		{endpoint: NewEndpoint("127.0.0.1:8333"), err: "connection refused"},
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
	closing  chan struct{}
	received chan struct{}
}

func newTestRecv(expected []byte) *testRecv {
	return &testRecv{
		expected: expected,
		closing:  make(chan struct{}),
		received: make(chan struct{}),
	}
}

func (t *testRecv) Receive(reader io.Reader) error {
	var buf [1024]byte
	n, err := reader.Read(buf[:])
	if err != nil {
		return err
	}
	t.buf.Write(buf[:n])
	if bytes.Equal(t.buf.Bytes(), t.expected) {
		close(t.received)
	}
	return nil
}

func (t *testRecv) Closing(_ error) {
	close(t.closing)
}

func randomData(n int) []byte {
	data := make([]byte, n)
	for i := 0; i < n; i++ {
		data[i] = 'a' + byte(rand.Intn(26))
	}
	return data
}
