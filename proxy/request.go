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
	"io"
	"sync"

	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/primitive"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

type request struct {
	client     *client
	session    *proxycore.Session
	idempotent bool
	done       bool
	retryCount int
	host       *proxycore.Host
	stream     int16
	qp         proxycore.QueryPlan
	raw        *frame.RawFrame
	mu         sync.Mutex
}

func (r *request) Execute(next bool) {
	r.mu.Lock()
	r.executeInternal(next)
	r.mu.Unlock()
}

// lock before using
func (r *request) executeInternal(next bool) {
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.idempotent {
		r.executeInternal(true)
	} else {
		if !r.done {
			r.done = true
			r.send(&message.Unavailable{ErrorMessage: "No more hosts available (cluster connection closed and request is not idempotent)"})
		}
	}
}

func (r *request) OnResult(raw *frame.RawFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done {
		if raw.Header.OpCode != primitive.OpCodeError ||
			!r.handleErrorResult(raw) { // If the error result is retried then we don't send back this response
			r.done = true
			r.sendRaw(raw)
		}
	}
}

func (r *request) handleErrorResult(raw *frame.RawFrame) (retried bool) {
	retried = false
	logger := r.client.proxy.logger
	decision := proxycore.ReturnError

	frm, err := codec.ConvertFromRawFrame(raw)
	if err != nil {
		logger.Error("unable to decode error frame for retry decision", zap.Error(err))
	} else if r.idempotent {
		errMsg := frm.Body.Message.(message.Error)
		logger.Debug("received error response",
			zap.Stringer("host", r.host),
			zap.Stringer("errorCode", errMsg.GetErrorCode()),
			zap.String("error", errMsg.GetErrorMessage()),
		)
		switch msg := frm.Body.Message.(type) {
		case *message.ReadTimeout:
			decision = r.client.proxy.config.RetryPolicy.OnReadTimeout(msg, r.retryCount)
			if decision != proxycore.ReturnError {
				logger.Debug("retrying read timeout",
					zap.Stringer("decision", decision),
					zap.Stringer("consistency", msg.Consistency),
					zap.Int32("requiredResponses", msg.BlockFor),
					zap.Int32("receivedResponses", msg.Received),
					zap.Bool("dataPresent", msg.DataPresent),
					zap.Int("retryCount", r.retryCount),
				)
			}
		case *message.WriteTimeout:
			decision = r.client.proxy.config.RetryPolicy.OnWriteTimeout(msg, r.retryCount)
			if decision != proxycore.ReturnError {
				logger.Debug("retrying write timeout",
					zap.Stringer("decision", decision),
					zap.Stringer("consistency", msg.Consistency),
					zap.Int32("requiredAcknowledgements", msg.BlockFor),
					zap.Int32("receivedAcknowledgements", msg.Received),
					zap.String("writeType", string(msg.WriteType)),
					zap.Int("retryCount", r.retryCount),
				)
			}
		case *message.Unavailable:
			decision = r.client.proxy.config.RetryPolicy.OnUnavailable(msg, r.retryCount)
			if decision != proxycore.ReturnError {
				logger.Debug("retrying on unavailable error",
					zap.Stringer("decision", decision),
					zap.Stringer("consistency", msg.Consistency),
					zap.Int32("aliveReplicas", msg.Alive),
					zap.Int32("requiredReplicas", msg.Required),
					zap.Int("retryCount", r.retryCount),
				)
			}
		case *message.IsBootstrapping:
			decision = proxycore.RetryNext
			logger.Debug("retrying on bootstrapping error",
				zap.Stringer("decision", decision),
				zap.Int("retryCount", r.retryCount),
			)
		case *message.ServerError, *message.Overloaded, *message.TruncateError,
			*message.ReadFailure, *message.WriteFailure:
			decision = r.client.proxy.config.RetryPolicy.OnErrorResponse(errMsg, r.retryCount)
			if decision != proxycore.ReturnError {
				logger.Debug("retrying on error response",
					zap.Stringer("decision", decision),
					zap.Int("retryCount", r.retryCount),
				)
			}
		default:
			// Do nothing, return the error
		}

		switch decision {
		case proxycore.RetryNext:
			r.retryCount++
			r.executeInternal(true)
			retried = true
		case proxycore.RetrySame:
			r.retryCount++
			r.executeInternal(false)
			retried = true
		default:
			// Do nothing, return the error
		}
	}

	return retried
}
