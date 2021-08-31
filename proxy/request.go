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

package proxy

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"sync"

	"cql-proxy/proxycore"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

type request struct {
	client     *client
	session    *proxycore.Session
	idempotent bool
	done       bool
	host       *proxycore.Host
	stream     int16
	qp         proxycore.QueryPlan
	raw        *frame.RawFrame
	mu         sync.Mutex
}

func (r *request) execute(next bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for !r.done {
		if next {
			r.host = r.qp.Next()
		}
		if r.host == nil {
			r.done = true
			r.send(&message.Unavailable{ErrorMessage: "No more hosts available (exhausted query plan)"})
		} else {
			err := r.session.Send(r.host, r)
			if err == nil {
				break
			} else {
				r.client.proxy.logger.Debug("failed to send request to host", zap.Stringer("host", r.host), zap.Error(err))
			}
		}
	}
}

func (r *request) send(msg message.Message) {
	_ = r.client.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(r.raw.Header.Version, r.stream, msg), writer)
	}))
}

func (r *request) sendRaw(raw *frame.RawFrame) {
	raw.Header.StreamId = r.stream
	_ = r.client.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeRawFrame(raw, writer)
	}))
}

func (r *request) Frame() interface{} {
	return r.raw
}

func (r *request) OnClose(_ error) {
	if r.idempotent {
		r.execute(true)
	} else {
		r.mu.Lock()
		if !r.done {
			r.done = true
			r.send(&message.Unavailable{ErrorMessage: "No more hosts available (cluster connection closed and request is not idempotent)"})
		}
		r.mu.Unlock()
	}
}

func readInt(bytes []byte) (int32, error) {
	if len(bytes) < 4 {
		return 0, errors.New("[int] expects at least 4 bytes")
	}
	return int32(binary.BigEndian.Uint32(bytes)), nil
}

func (r *request) OnResult(raw *frame.RawFrame) {
	switch raw.Header.OpCode {
	case primitive.OpCodeError:
		if r.handleErrorResponse(raw) {
			return
		}
	case primitive.OpCodeResult:
		r.handleResultResponse(raw)
	}
	r.sendResult(raw)
}

func (r *request) handleErrorResponse(raw *frame.RawFrame) bool {
	code, err := readInt(raw.Body)
	if err != nil {
		r.client.proxy.logger.Error("failed to read `code` in error response", zap.Error(err))
		return false
	}

	if primitive.ErrorCode(code) == primitive.ErrorCodeUnprepared {
		frm, err := codec.ConvertFromRawFrame(raw)
		if err != nil {
			r.client.proxy.logger.Error("failed to decode unprepared error response", zap.Error(err))
			return false
		}
		msg := frm.Body.Message.(*message.Unprepared)
		id := hex.EncodeToString(msg.Id)
		if prepare, ok := r.client.proxy.preparedCache.Load(id); ok {
			err := r.session.Send(r.host, &prepareRequest{
				prepare:     prepare.(*frame.RawFrame),
				origRequest: r,
			})
			if err != nil {
				r.client.proxy.logger.Error("failed to re-prepared query after receiving an unprepared error response",
					zap.String("host", r.host.String()),
					zap.String("id", id),
					zap.Error(err))
				return false
			}
			return true
		} else {
			r.client.proxy.logger.Warn("received unprepared error response, but existing prepared ID not in the cache",
				zap.String("id", id))
		}
	}

	return false
}

func (r *request) handleResultResponse(raw *frame.RawFrame) {
	kind, err := readInt(raw.Body)
	if err != nil {
		r.client.proxy.logger.Error("failed to read `kind` in result response", zap.Error(err))
		return
	}
	if primitive.ResultType(kind) == primitive.ResultTypePrepared {
		frm, err := codec.ConvertFromRawFrame(raw)
		if err != nil {
			r.client.proxy.logger.Error("failed to decode prepared result response", zap.Error(err))
			return
		}
		msg := frm.Body.Message.(*message.PreparedResult)
		r.client.proxy.preparedCache.Store(hex.EncodeToString(msg.PreparedQueryId), r.raw) // Store frame so we can re-prepare
	}
}

func (r *request) sendResult(raw *frame.RawFrame) {
	r.mu.Lock()
	if !r.done {
		r.done = true
		r.sendRaw(raw)
	}
	r.mu.Unlock()
}

type prepareRequest struct {
	prepare     *frame.RawFrame
	origRequest *request
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
		r.origRequest.client.proxy.logger.Error("unable to re-prepare request") // TODO: Add host and error
		next = true                                                             // Try the next node
	}
	r.origRequest.execute(next)
}
