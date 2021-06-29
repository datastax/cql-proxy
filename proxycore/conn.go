// Copyright 2020 DataStax
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
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
)

var (
	Closed        = errors.New("connection closed")
	AlreadyClosed = errors.New("connection already closed")
)

const (
	MaxMessages     = 1024
	MaxCoalesceSize = 16 * 1024
)

type Conn struct {
	conn     net.Conn
	closed   chan struct{}
	messages chan Sender
	err      error
	recv     Receiver
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

func Connect(ctx context.Context, endpoint Endpoint, recv Receiver) (*Conn, error) {
	var addr string
	if endpoint.IsResolved() {
		addr = endpoint.Addr()
	} else {
		parts := strings.Split(endpoint.Addr(), ":")
		addrs, err := net.LookupHost(parts[0])
		if err != nil {
			return nil, err
		}
		addr = addrs[rand.Intn(len(addrs))]
		if len(parts) > 1 {
			addr = fmt.Sprintf("%s:%s", addr, parts[1])
		}

	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	if endpoint.TlsConfig() != nil {
		tlsConn := tls.Client(conn, endpoint.TlsConfig())
		if err = tlsConn.Handshake(); err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	c := NewConn(conn, recv)
	c.Start()
	return c, nil
}

func NewConn(conn net.Conn, recv Receiver) *Conn {
	return &Conn{
		conn:     conn,
		recv:     recv,
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
		done = c.checkErr(c.recv.Receive(c.conn))
	}
	c.recv.Closing(c.Err())
	//log.Println("reader closed")
}

func (c *Conn) write() {
	done := false
	writer := bytes.NewBuffer(make([]byte, 0))
	senders := make([]Sender, 0)

	for {
		select {
		case sender := <-c.messages:
			done = c.checkErr(sender.Send(writer))
			if !done {
				senders = append(senders, sender)
			}
			coalescing := true
			for coalescing && !done && writer.Len() < MaxCoalesceSize {
				select {
				case sender, coalescing = <-c.messages:
					done = c.checkErr(sender.Send(writer))
					if !done {
						senders = append(senders, sender)
					}
				case <-c.closed:
					done = true
				default:
					coalescing = false
				}
			}
		case <-c.closed:
			done = true
		}

		_, err := c.conn.Write(writer.Bytes())
		done = c.checkErr(err)
		if done {
			break
		}
		//log.Printf("wrote %d bytes, %d senders", n, len(senders))
		senders = senders[:0]
		writer.Reset()
	}
	//log.Println("writer closed")
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
