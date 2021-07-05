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
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io"
	"strings"
	"sync/atomic"
)

const (
	MaxStreams = 2048
)

var allEvents = []primitive.EventType{primitive.EventTypeSchemaChange, primitive.EventTypeTopologyChange, primitive.EventTypeStatusChange}

type Request interface {
	Frame() interface{}
	OnClose(err error)
	OnResult(raw *frame.RawFrame)
}

type EventHandler interface {
	OnEvent(frm *frame.Frame)
}

type ClientConn struct {
	conn         *Conn
	inflight     int32
	pending      *pendingRequests
	eventHandler EventHandler
}

func ConnectClient(ctx context.Context, endpoint Endpoint) (*ClientConn, error) {
	return ConnectClientWithEvents(ctx, endpoint, nil)
}

func ConnectClientWithEvents(ctx context.Context, endpoint Endpoint, handler EventHandler) (*ClientConn, error) {
	c := &ClientConn{
		conn:         nil,
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
			if c.eventHandler != nil {
				return version, c.registerForEvents(ctx, version)
			}
			return version, nil
		case *message.Authenticate:
			if auth == nil {
				return version, AuthExpected
			}
			err = c.authInitialResponse(ctx, version, auth, msg)
			if err == nil && c.eventHandler != nil {
				return version, c.registerForEvents(ctx, version)
			}
			return version, err
		case message.Error:
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
			return version, &CqlError{Message: msg}
		default:
			return version, &UnexpectedResponse{
				Expected: []string{"READY", "AUTHENTICATE"},
				Received: response.Body.String(),
			}
		}
	}
}

func (c *ClientConn) registerForEvents(ctx context.Context, version primitive.ProtocolVersion) error {
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.Register{EventTypes: allEvents}))
	if err != nil {
		return err
	}

	switch msg := response.Body.Message.(type) {
	case *message.Ready:
		return nil
	case message.Error:
		return &CqlError{Message: msg}
	default:
		return &UnexpectedResponse{
			Expected: []string{"READY"},
			Received: response.Body.String(),
		}
	}
}

func (c *ClientConn) authInitialResponse(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, authenticate *message.Authenticate) error {
	token, err := auth.InitialResponse(authenticate.Authenticator)
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
	case message.Error:
		return &CqlError{Message: msg}
	default:
		return &UnexpectedResponse{
			Expected: []string{"AUTH_CHALLENGE", "AUTH_SUCCESS"},
			Received: response.Body.String(),
		}
	}
}

func (c *ClientConn) authChallenge(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, challenge *message.AuthChallenge) error {
	token, err := auth.EvaluateChallenge(challenge.Token)
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
	case message.Error:
		return &CqlError{Message: msg}
	default:
		return &UnexpectedResponse{
			Expected: []string{"AUTH_SUCCESS"},
			Received: response.Body.String(),
		}
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
	case *message.VoidResult:
		return nil, nil // TODO: Make empty result set
	case message.Error:
		return nil, &CqlError{Message: msg}
	default:
		return nil, &UnexpectedResponse{
			Expected: []string{"RESULT(Rows)", "RESULT(Void)"},
			Received: response.Body.String(),
		}
	}
}

func (c *ClientConn) SetKeyspace(ctx context.Context, version primitive.ProtocolVersion, keyspace string) error {
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.Query{
		Query: fmt.Sprintf("USE %s", keyspace),
	}))
	if err != nil {
		return err
	}

	switch msg := response.Body.Message.(type) {
	case *message.SetKeyspaceResult:
		return nil
	case message.Error:
		return &CqlError{Message: msg}
	default:
		return &UnexpectedResponse{
			Expected: []string{"RESULT(Set_Keyspace)"},
			Received: response.Body.String(),
		}
	}
}

func (c *ClientConn) Receive(reader io.Reader) error {
	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		return fmt.Errorf("unable to decode frame: %w", err)
	}

	if raw.Header.OpCode == primitive.OpCodeEvent {
		if c.eventHandler != nil {
			frm, err := codec.ConvertFromRawFrame(raw)
			if err != nil {
				return fmt.Errorf("unable to convert raw event frame: %w", err)
			}
			c.eventHandler.OnEvent(frm)
		}
	} else {
		request := c.pending.loadAndDelete(raw.Header.StreamId)
		if request == nil {
			return errors.New("invalid stream")
		}
		atomic.AddInt32(&c.inflight, -1)
		request.OnResult(raw)
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
		frame: f,
		err:   make(chan error),
		res:   make(chan *frame.RawFrame),
	}

	err := c.Send(request)
	if err != nil {
		return nil, err
	}

	select {
	case r := <-request.res:
		return codec.ConvertFromRawFrame(r)
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
		return codec.EncodeFrame(frm, writer)
	case *frame.RawFrame:
		frm.Header.StreamId = stream
		return codec.EncodeRawFrame(frm, writer)
	default:
		return errors.New("unhandled frame type")
	}
}

type internalRequest struct {
	frame *frame.Frame
	err   chan error
	res   chan *frame.RawFrame
}

func (i *internalRequest) Frame() interface{} {
	return i.frame
}

func (i *internalRequest) OnClose(err error) {
	i.err <- err
}

func (i *internalRequest) OnResult(raw *frame.RawFrame) {
	i.res <- raw
}
