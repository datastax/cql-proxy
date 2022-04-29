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
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
)

var (
	Closed        = errors.New("connection closed")
	AlreadyClosed = errors.New("connection already closed")
)

const (
	MaxMessages     = 1024
	MaxCoalesceSize = 16 * 1024 // TODO: What's a good value for this?
)

type Conn struct {
	conn     net.Conn
	closed   chan struct{}
	messages chan Sender
	err      error
	recv     Receiver
	writer   *bufio.Writer
	reader   *bufio.Reader
	mu       *sync.Mutex
}

type Receiver interface {
	Receive(reader io.Reader) error
	Closing(err error)
}

type Sender interface {
	Send(writer io.Writer) error
}

type SenderFunc func(writer io.Writer) error

func (s SenderFunc) Send(writer io.Writer) error {
	return s(writer)
}

// Connect creates a new connection to a server specified by the endpoint using TLS if specified
func Connect(ctx context.Context, endpoint Endpoint, recv Receiver) (c *Conn, err error) {
	var dialer net.Dialer
	addr, err := LookupEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()

	if endpoint.TLSConfig() != nil {
		tlsConn := tls.Client(conn, endpoint.TLSConfig())
		if err = tlsConn.Handshake(); err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	c = NewConn(conn, recv)
	c.Start()
	return c, nil
}

func NewConn(conn net.Conn, recv Receiver) *Conn {
	return &Conn{
		conn:     conn,
		recv:     recv,
		writer:   bufio.NewWriterSize(conn, MaxCoalesceSize),
		reader:   bufio.NewReader(conn),
		closed:   make(chan struct{}),
		messages: make(chan Sender, MaxMessages),
		mu:       &sync.Mutex{},
	}
}

func (c *Conn) Start() {
	go c.read()
	go c.write()
}

func (c *Conn) read() {
	done := false
	for !done {
		done = c.checkErr(c.recv.Receive(c.reader))
	}
	c.recv.Closing(c.Err())
}

func (c *Conn) write() {
	done := false

	for !done {
		select {
		case sender := <-c.messages:
			done = c.checkErr(sender.Send(c.writer))
			coalescing := true
			for coalescing && !done {
				select {
				case sender, coalescing = <-c.messages:
					done = c.checkErr(sender.Send(c.writer))
				case <-c.closed:
					done = true
				default:
					coalescing = false
				}
			}
		case <-c.closed:
			done = true
		}

		if !done { // Check to avoid resetting `done` to false
			err := c.writer.Flush()
			done = c.checkErr(err)
		}
	}
}

func (c *Conn) WriteBytes(b []byte) error {
	return c.Write(SenderFunc(func(writer io.Writer) error {
		_, err := writer.Write(b)
		return err
	}))
}

func (c *Conn) Write(sender Sender) error {
	select {
	case c.messages <- sender:
		return nil
	case <-c.closed:
		return c.Err()
	}
}

func (c *Conn) checkErr(err error) bool {
	if err != nil {
		c.mu.Lock()
		if c.err == nil {
			c.err = err
			_ = c.conn.Close()
			close(c.closed)
		}
		c.mu.Unlock()
		return true
	}
	return false
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return AlreadyClosed
	}
	close(c.closed)
	c.err = Closed
	return c.conn.Close()
}

func (c *Conn) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

func (c *Conn) IsClosed() chan struct{} {
	return c.closed
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
