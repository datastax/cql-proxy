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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"

	"go.uber.org/zap"
)

const (
	MaxStreams = 2048
)

var allEvents = []primitive.EventType{primitive.EventTypeSchemaChange, primitive.EventTypeTopologyChange, primitive.EventTypeStatusChange}

type EventHandler interface {
	OnEvent(frm *frame.Frame)
}

type EventHandlerFunc func(frm *frame.Frame)

func (f EventHandlerFunc) OnEvent(frm *frame.Frame) {
	f(frm)
}

type ClientConnConfig struct {
	PreparedCache PreparedCache
	Handler       EventHandler
	Logger        *zap.Logger
}

type ClientConn struct {
	conn          *Conn
	inflight      int32
	pending       *pendingRequests
	eventHandler  EventHandler
	preparedCache PreparedCache
	logger        *zap.Logger
	closing       bool
	closingMu     *sync.RWMutex
}

// ConnectClient creates a new connection to an endpoint within a downstream cluster using TLS if specified.
func ConnectClient(ctx context.Context, endpoint Endpoint, config ClientConnConfig) (*ClientConn, error) {
	c := &ClientConn{
		pending:       newPendingRequests(MaxStreams),
		eventHandler:  config.Handler,
		closingMu:     &sync.RWMutex{},
		preparedCache: config.PreparedCache,
		logger:        GetOrCreateNopLogger(config.Logger),
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
		return err
	}

	if raw.Header.OpCode == primitive.OpCodeEvent {
		if c.eventHandler != nil {
			frm, err := codec.ConvertFromRawFrame(raw)
			if err != nil {
				return err
			}
			c.eventHandler.OnEvent(frm)
		}
	} else {
		request := c.pending.loadAndDelete(raw.Header.StreamId)
		if request == nil {
			return errors.New("invalid stream")
		}
		atomic.AddInt32(&c.inflight, -1)

		handled := false

		// If we have a prepared cache attempt to recover from unprepared errors and cache previously seen prepared
		// requests (so they can be used to prepare other nodes).
		if c.preparedCache != nil {
			switch raw.Header.OpCode {
			case primitive.OpCodeError:
				handled = c.maybePrepareAndExecute(request, raw)
			case primitive.OpCodeResult:
				c.maybeCachePrepared(request, raw)
			}
		}

		if !handled {
			request.OnResult(raw)
		}
	}

	return nil
}

// maybePrepareAndExecute checks the response looking for unprepared errors and attempts to prepare them.
// If an unprepared error is encountered it attempts to prepare the query on the connection and re-execute the original
// request.
func (c *ClientConn) maybePrepareAndExecute(request Request, raw *frame.RawFrame) bool {
	code, err := readInt(raw.Body)
	if err != nil {
		c.logger.Error("failed to read `code` in error response", zap.Error(err))
		return false
	}

	if primitive.ErrorCode(code) == primitive.ErrorCodeUnprepared {
		frm, err := codec.ConvertFromRawFrame(raw)
		if err != nil {
			c.logger.Error("failed to decode unprepared error response", zap.Error(err))
			return false
		}
		msg := frm.Body.Message.(*message.Unprepared)
		id := hex.EncodeToString(msg.Id)
		if prepare, ok := c.preparedCache.Load(id); ok {
			err = c.Send(&prepareRequest{
				prepare:     prepare.PreparedFrame,
				origRequest: request,
			})
			if err != nil {
				c.logger.Error("failed to prepare query after receiving an unprepared error response",
					zap.String("host", c.conn.RemoteAddr().String()),
					zap.String("id", id),
					zap.Error(err))
				return false
			} else {
				return true
			}
		} else {
			c.logger.Warn("received unprepared error response, but existing prepared ID not in the cache",
				zap.String("id", id))
		}
	}
	return false
}

// maybeCachePrepared checks the response looking for prepared frames and caches the original prepare request.
// This is done so that the prepare request can be used to prepare other nodes that have not been prepared, but are
// attempting to execute a request that has been prepared on another node in the cluster.
func (c *ClientConn) maybeCachePrepared(request Request, raw *frame.RawFrame) {
	kind, err := readInt(raw.Body)
	if err != nil {
		c.logger.Error("failed to read `kind` in result response", zap.Error(err))
		return
	}
	if primitive.ResultType(kind) == primitive.ResultTypePrepared {
		frm, err := codec.ConvertFromRawFrame(raw)
		if err != nil {
			c.logger.Error("failed to decode prepared result response", zap.Error(err))
			return
		}
		msg := frm.Body.Message.(*message.PreparedResult)
		c.preparedCache.Store(hex.EncodeToString(msg.PreparedQueryId),
			&PreparedEntry{
				request.Frame().(*frame.RawFrame), // Store frame so we can re-prepare
			})
	}
}

func (c *ClientConn) Closing(err error) {
	c.closingMu.Lock()
	c.closing = true
	c.pending.closing(err)
	c.closingMu.Unlock()
}

func (c *ClientConn) addToPending(request Request) (int16, error) {
	c.closingMu.RLock()
	defer c.closingMu.RUnlock()
	if c.closing {
		return 0, Closed
	}
	stream := c.pending.store(request)
	if stream < 0 {
		return 0, StreamsExhausted
	}
	return stream, nil
}

func (c *ClientConn) Send(request Request) error {
	stream, err := c.addToPending(request)
	if err != nil {
		return err
	}

	err = c.conn.Write(&requestSender{
		request: request,
		stream:  stream,
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
		err:   make(chan error, 1),
		res:   make(chan *frame.RawFrame, 1),
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

// Heartbeats sends an OPTIONS request to the endpoint in order to keep the connection alive.
func (c *ClientConn) Heartbeats(connectTimeout time.Duration, version primitive.ProtocolVersion, heartbeatInterval time.Duration, idleTimeout time.Duration, logger *zap.Logger) {
	idleTimer := time.NewTimer(idleTimeout)

	for {
		select {
		case <-c.conn.IsClosed():
			return
		case <-time.After(heartbeatInterval):
			ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
			response, err := c.SendAndReceive(ctx, frame.NewFrame(version, -1, &message.Options{}))
			cancel()
			if err != nil {
				logger.Warn("error occurred performing heartbeat", zap.Error(err))
				continue
			}

			switch response.Body.Message.(type) {
			case *message.Supported:
				logger.Debug("successfully performed a heartbeat", zap.Stringer("remoteAddress", c.conn.RemoteAddr()))
				if idleTimer.Stop() {
					idleTimer.Reset(idleTimeout)
				}
			case message.Error:
				logger.Warn("error occurred performing heartbeat", zap.String("optionsError", response.Body.String()))
			default:
				logger.Warn("unexpected message received while performing heartbeat", zap.String("optionsError", response.Body.String()))
			}
		case <-idleTimer.C:
			_ = c.Close()
			logger.Sugar().Errorf("error connection didn't perform heartbeats within %v", idleTimeout)
			return
		}
	}
}

type requestSender struct {
	request Request
	stream  int16
	conn    *ClientConn
}

func (r *requestSender) Send(writer io.Writer) error {
	switch frm := r.request.Frame().(type) {
	case *frame.Frame:
		frm.Header.StreamId = r.stream
		return codec.EncodeFrame(frm, writer)
	case *frame.RawFrame:
		frm.Header.StreamId = r.stream
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

func (i *internalRequest) Execute(_ bool) {
	panic("not implemented")
}

func (i *internalRequest) Frame() interface{} {
	return i.frame
}

func (i *internalRequest) OnClose(err error) {
	select {
	case i.err <- err:
	default:
		panic("attempted to close request multiple times")
	}
}

func (i *internalRequest) OnResult(raw *frame.RawFrame) {
	select {
	case i.res <- raw:
	default:
		panic("attempted to set result multiple times")
	}
}

type prepareRequest struct {
	prepare     *frame.RawFrame
	origRequest Request
}

func (r *prepareRequest) Execute(_ bool) {
	panic("not implemented")
}

func (r *prepareRequest) Frame() interface{} {
	return r.prepare
}

func (r *prepareRequest) OnClose(err error) {
	r.origRequest.OnClose(err)
}

func (r *prepareRequest) OnResult(raw *frame.RawFrame) {
	next := false // If there's no error then we re-try on the original host
	if raw.Header.OpCode == primitive.OpCodeError {
		next = true // Try the next node
	}
	r.origRequest.Execute(next)
}

func readInt(bytes []byte) (int32, error) {
	if len(bytes) < 4 {
		return 0, errors.New("[int] expects at least 4 bytes")
	}
	return int32(binary.BigEndian.Uint32(bytes)), nil
}
