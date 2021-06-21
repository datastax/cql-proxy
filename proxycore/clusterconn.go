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
	"log"
)

var (
	InvalidStream    = errors.New("invalid stream")
	StreamsExhausted = errors.New("streams exhausted")
)

const (
	MaxStreams = 2048
)

type ClusterRequest interface {
	Frame() *frame.Frame
	OnError(err error)
	OnResult(frame *frame.Frame)
}

type ClusterConn struct {
	conn    *Conn
	codec   frame.Codec
	pending *pendingRequests
}

type Authenticator interface {
	InitialResponse(authenticator string) ([]byte, error)
	EvaluateChallenge(token []byte) ([]byte, error)
	Success(token []byte) error
}

func ClusterConnect(ctx context.Context, endpoint Endpoint) (*ClusterConn, error) {
	c := &ClusterConn{
		conn:    nil,
		codec:   frame.NewRawCodec(), // TODO
		pending: newPendingRequests(MaxStreams),
	}
	var err error
	c.conn, err = Connect(ctx, endpoint, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ClusterConn) Handshake(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator) error {
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, message.NewStartup()))
	if err != nil {
		return err
	}

	switch msg := response.Body.Message.(type) {
	case *message.Ready:
	case *message.Authenticate:
		if auth == nil {
			return errors.New("authentication required, but no authenticator provided")
		}
		return c.authInitialResponse(ctx, version, auth, msg)
	default:
		return fmt.Errorf("expected READY or AUTHENTICATE response types, got: %v", response.Body.Message)
	}

	return nil
}

func (c *ClusterConn) authInitialResponse(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, msg *message.Authenticate) error {
	token, err := auth.InitialResponse(msg.Authenticator)
	if err != nil {
		return err
	}
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.AuthResponse{Token: token}))

	switch msg := response.Body.Message.(type) {
	case *message.AuthChallenge:
		return c.authChallenge(ctx, version, auth, msg)
	case *message.AuthSuccess:
		return auth.Success(msg.Token)
	default:
		return fmt.Errorf("expected AUTH_CHALLENGE or AUTH_SUCCESS response types, got: %v", response.Body.Message)
	}
}

func (c *ClusterConn) authChallenge(ctx context.Context, version primitive.ProtocolVersion, auth Authenticator, msg *message.AuthChallenge) error {
	token, err := auth.EvaluateChallenge(msg.Token)
	if err != nil {
		return err
	}
	response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.AuthResponse{Token: token}))

	switch msg := response.Body.Message.(type) {
	case *message.AuthSuccess:
		return auth.Success(msg.Token)
	default:
		return fmt.Errorf("expected AUTH_SUCCESS response type, got: %v", response.Body.Message)
	}
}

func (c *ClusterConn) Receive(reader io.Reader) error {
	frame, err := c.codec.DecodeFrame(reader)
	if err != nil {
		return err
	}

	if frame.Header.OpCode == primitive.OpCodeEvent {
		// TODO
		log.Fatalln("events not handled yet")
	} else {
		request := c.pending.loadAndDelete(frame.Header.StreamId)
		if request == nil {
			return InvalidStream
		}
		request.OnResult(frame)
	}

	return nil
}

func (c *ClusterConn) Closing(err error) {
	c.pending.sendError(err)
}

func (c *ClusterConn) Send(request ClusterRequest) error {
	return c.conn.Write(&requestSender{
		request: request,
		conn:    c,
	})
}

func (c *ClusterConn) SendAndReceive(ctx context.Context, f *frame.Frame) (*frame.Frame, error) {
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
	case <- ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ClusterConn) Close() error {
	return c.conn.Close()
}

func (c *ClusterConn) IsClosed() chan struct{} {
	return c.conn.IsClosed()
}

func (c *ClusterConn) Err() error {
	return c.conn.Err()
}

type requestSender struct {
	request ClusterRequest
	conn    *ClusterConn
}

func (r *requestSender) Send(writer io.Writer) error {
	stream := r.conn.pending.store(r.request)
	if stream < 0 {
		return StreamsExhausted
	}
	r.request.Frame().Header.StreamId = stream
	return r.conn.codec.EncodeFrame(r.request.Frame(), writer)
}

type internalRequest struct {
	frame  *frame.Frame
	err    chan error
	result chan *frame.Frame
}

func (i *internalRequest) Frame() *frame.Frame {
	return i.frame
}

func (i *internalRequest) OnError(err error) {
	i.err <- err
}

func (i *internalRequest) OnResult(frame *frame.Frame) {
	i.result <- frame
}
