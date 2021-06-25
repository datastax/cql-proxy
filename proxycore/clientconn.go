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
	"context"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io"
	"strings"
	"sync/atomic"
)

var (
	InvalidStream    = errors.New("invalid stream")
	StreamsExhausted = errors.New("streams exhausted")
)

const (
	MaxStreams = 2048
)

type Request interface {
	Frame() interface{}
	OnError(err error)
	OnResult(frame *frame.Frame)
}

type EventHandler interface {
	OnEvent(frame *frame.Frame)
}

type ClientConn struct {
	conn         *Conn
	inflight     int32
	codec        frame.RawCodec
	pending      *pendingRequests
	eventHandler EventHandler
}

type Authenticator interface {
	InitialResponse(authenticator string) ([]byte, error)
	EvaluateChallenge(token []byte) ([]byte, error)
	Success(token []byte) error
}

func ConnectClient(ctx context.Context, endpoint Endpoint) (*ClientConn, error) {
	return ConnectClientWithEvents(ctx, endpoint, nil)
}

func ConnectClientWithEvents(ctx context.Context, endpoint Endpoint, handler EventHandler) (*ClientConn, error) {
	c := &ClientConn{
		conn:         nil,
		codec:        frame.NewRawCodec(), // TODO
		pending:      newPendingRequests(MaxStreams),
		eventHandler: handler,
	}
	var err error
	c.conn, err = Connect(ctx, endpoint, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ClientConn) Handshake(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator) (primitive.ProtocolVersion, error) {
	for {
		response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, message.NewStartup()))
		if err != nil {
			return version, err
		}

		switch msg := response.Body.Message.(type) {
		case *message.Ready:
			return version, nil
		case *message.Authenticate:
			if auth == nil {
				return version, errors.New("authentication required, but no authenticator provided")
			}
			return version, c.authInitialResponse(ctx, version, auth, msg)
		default:
			if pe, ok := msg.(*message.ProtocolError); ok {
				if strings.Contains(pe.ErrorMessage, "Invalid or unsupported protocol version") {
					switch version {
					case primitive.ProtocolVersionDse2:
						version = primitive.ProtocolVersionDse1
						continue
					case primitive.ProtocolVersionDse1:
						version = primitive.ProtocolVersion4
						continue
					case primitive.ProtocolVersion2:
					default:
						version--
						continue
					}
				}
			}
			return version, fmt.Errorf("expected READY or AUTHENTICATE response types, got: %v", response.Body.Message)
		}
	}
}

func (c *ClientConn) authInitialResponse(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, msg *message.Authenticate) error {
	token, err := auth.InitialResponse(msg.Authenticator)
	if err != nil {
		return err
	}
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.AuthResponse{Token: token}))
	if err != nil {
		return err
	}

	switch msg := response.Body.Message.(type) {
	case *message.AuthChallenge:
		return c.authChallenge(ctx, version, auth, msg)
	case *message.AuthSuccess:
		return auth.Success(msg.Token)
	default:
		return fmt.Errorf("expected AUTH_CHALLENGE or AUTH_SUCCESS response types, got: %v", response.Body.Message)
	}
}

func (c *ClientConn) authChallenge(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, msg *message.AuthChallenge) error {
	token, err := auth.EvaluateChallenge(msg.Token)
	if err != nil {
		return err
	}
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.AuthResponse{Token: token}))
	if err != nil {
		return err
	}

	switch msg := response.Body.Message.(type) {
	case *message.AuthSuccess:
		return auth.Success(msg.Token)
	default:
		return fmt.Errorf("expected AUTH_SUCCESS response type, got: %v", response.Body.Message)
	}
}

func (c *ClientConn) Inflight() int32 {
	return atomic.LoadInt32(&c.inflight)
}

func (c *ClientConn) Query(ctx context.Context, version primitive.ProtocolVersion, query *message.Query) (*ResultSet, error) {
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, query))
	if err != nil {
		return nil, err
	}

	switch msg := response.Body.Message.(type) {
	case *message.RowsResult:
		return NewResultSet(msg, version), nil
	default:
		return nil, fmt.Errorf("expected rows response type, got: %v", response.Body.Message)
	}
}

func (c *ClientConn) Receive(reader io.Reader) error {
	decoded, err := c.codec.DecodeFrame(reader)
	if err != nil {
		return err
	}

	if decoded.Header.OpCode == primitive.OpCodeEvent {
		if c.eventHandler != nil {
			c.eventHandler.OnEvent(decoded)
		}
	} else {
		request := c.pending.loadAndDelete(decoded.Header.StreamId)
		if request == nil {
			return InvalidStream
		}
		atomic.AddInt32(&c.inflight, -1)
		request.OnResult(decoded)
	}

	return nil
}

func (c *ClientConn) Closing(err error) {
	c.pending.sendError(err)
}

func (c *ClientConn) Send(request Request) error {
	err := c.conn.Write(&requestSender{
		request: request,
		conn:    c,
	})
	if err == nil {
		atomic.AddInt32(&c.inflight, 1)
	}
	return err
}

func (c *ClientConn) SendAndReceive(ctx context.Context, f *frame.Frame) (*frame.Frame, error) {
	request := &internalRequest{
		frame:  f,
		err:    make(chan error),
		result: make(chan *frame.Frame),
	}

	err := c.Send(request)
	if err != nil {
		return nil, err
	}

	select {
	case r := <-request.result:
		return r, nil
	case e := <-request.err:
		return nil, e
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ClientConn) Close() error {
	return c.conn.Close()
}

func (c *ClientConn) IsClosed() chan struct{} {
	return c.conn.IsClosed()
}

func (c *ClientConn) Err() error {
	return c.conn.Err()
}

type requestSender struct {
	request Request
	conn    *ClientConn
}

func (r *requestSender) Send(writer io.Writer) error {
	stream := r.conn.pending.store(r.request)
	if stream < 0 {
		return StreamsExhausted
	}
	switch frm := r.request.Frame().(type) {
	case *frame.Frame:
		frm.Header.StreamId = stream
		return r.conn.codec.EncodeFrame(frm, writer)
	case *frame.RawFrame:
		frm.Header.StreamId = stream
		return r.conn.codec.EncodeRawFrame(frm, writer)
	default:
		return errors.New("unhandled frame type")
	}
}

type internalRequest struct {
	frame  *frame.Frame
	err    chan error
	result chan *frame.Frame
}

func (i *internalRequest) Frame() interface{} {
	return i.frame
}

func (i *internalRequest) OnError(err error) {
	i.err <- err
}

func (i *internalRequest) OnResult(frame *frame.Frame) {
	i.result <- frame
}
