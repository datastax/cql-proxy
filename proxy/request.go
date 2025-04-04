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
	"errors"
	"io"
	"reflect"
	"sync"

	"github.com/datastax/cql-proxy/codecs"
	"github.com/datastax/cql-proxy/parser"
	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/primitive"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

type idempotentState int

const (
	notDetermined idempotentState = iota
	notIdempotent
	isIdempotent
)

type request struct {
	client     *client
	session    *proxycore.Session
	state      idempotentState
	keyspace   string
	msg        message.Message
	done       bool
	retryCount int
	host       *proxycore.Host
	stream     int16
	version    primitive.ProtocolVersion
	qp         proxycore.QueryPlan
	frm        interface{}
	isSelect   bool // Only used for prepared statements currently
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
			r.send(&message.ServerError{ErrorMessage: "Proxy exhausted query plan and there are no more hosts available to try"})
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
		return r.client.codec.EncodeFrame(frame.NewFrame(r.version, r.stream, msg), writer)
	}))
}

func (r *request) sendRaw(raw *frame.RawFrame) {
	raw.Header.StreamId = r.stream
	_ = r.client.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return r.client.codec.EncodeRawFrame(raw, writer)
	}))
}

func (r *request) Frame() interface{} {
	return r.frm
}
func (r *request) IsPrepareRequest() bool {
	_, isPrepare := r.msg.(*message.Prepare)
	return isPrepare
}

func (r *request) checkIdempotent() bool {
	if notDetermined == r.state {
		idempotent := false
		var err error
		if r.msg != nil {
			switch msg := r.msg.(type) {
			case *codecs.PartialQuery:
				idempotent, err = parser.IsQueryIdempotent(msg.Query)
			case *codecs.PartialExecute:
				idempotent = r.client.proxy.isIdempotent(msg.QueryId)
			case *codecs.PartialBatch:
				idempotent, err = r.isBatchIdempotent(msg)
			default:
				r.client.proxy.logger.Error("invalid message type encountered when checking for idempotence",
					zap.Stringer("type", reflect.TypeOf(msg)))
			}
		}
		if err != nil {
			r.client.proxy.logger.Error("error parsing query for idempotence",
				zap.Error(err),
				zap.Stringer("type", reflect.TypeOf(r.msg)))
		}
		if idempotent {
			r.state = isIdempotent
		} else {
			r.state = notIdempotent
		}
	}
	return isIdempotent == r.state
}

func (r *request) OnClose(_ error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.checkIdempotent() {
		r.executeInternal(true)
	} else {
		if !r.done {
			r.done = true
			r.send(&message.ServerError{ErrorMessage: "Proxy is unable to retry non-idempotent query after connection to backend cluster closed"})
		}
	}
}

func (r *request) OnResult(raw *frame.RawFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done {
		if raw.Header.OpCode != primitive.OpCodeError ||
			!r.handleErrorResult(raw) { // If the error result is retried then we don't send back this response
			r.client.maybeStorePreparedMetadata(raw, r.isSelect, r.msg)
			r.done = true
			r.sendRaw(raw)
		}
	}
}

func (r *request) handleErrorResult(raw *frame.RawFrame) (retried bool) {
	retried = false
	logger := r.client.proxy.logger
	decision := ReturnError

	frm, err := r.client.codec.ConvertFromRawFrame(raw)
	if err != nil {
		logger.Error("unable to decode error frame for retry decision", zap.Error(err))
	} else {
		errMsg := frm.Body.Message.(message.Error)

		logger.Debug("received error response",
			zap.Stringer("host", r.host),
			zap.Stringer("errorCode", errMsg.GetErrorCode()),
			zap.String("error", errMsg.GetErrorMessage()),
		)
		switch msg := frm.Body.Message.(type) {
		case *message.ReadTimeout:
			decision = r.client.proxy.config.RetryPolicy.OnReadTimeout(msg, r.retryCount)
			if decision != ReturnError {
				logger.Debug("retrying read timeout",
					zap.Stringer("decision", decision),
					zap.Stringer("response", msg),
					zap.Int("retryCount", r.retryCount),
				)
			}
		case *message.WriteTimeout:
			if r.checkIdempotent() {
				decision = r.client.proxy.config.RetryPolicy.OnWriteTimeout(msg, r.retryCount)
				if decision != ReturnError {
					logger.Debug("retrying write timeout",
						zap.Stringer("decision", decision),
						zap.Stringer("response", msg),
						zap.Int("retryCount", r.retryCount),
					)
				}
			}
		case *message.Unavailable:
			decision = r.client.proxy.config.RetryPolicy.OnUnavailable(msg, r.retryCount)
			if decision != ReturnError {
				logger.Debug("retrying on unavailable error",
					zap.Stringer("decision", decision),
					zap.Stringer("response", msg),
					zap.Int("retryCount", r.retryCount),
				)
			}
		case *message.IsBootstrapping:
			decision = RetryNext
			logger.Debug("retrying on bootstrapping error",
				zap.Stringer("decision", decision),
				zap.Int("retryCount", r.retryCount),
			)
		case *message.ServerError, *message.Overloaded, *message.TruncateError,
			*message.ReadFailure, *message.WriteFailure:
			if r.checkIdempotent() {
				decision = r.client.proxy.config.RetryPolicy.OnErrorResponse(errMsg, r.retryCount)
				if decision != ReturnError {
					logger.Debug("retrying on error response",
						zap.Stringer("decision", decision),
						zap.Int("retryCount", r.retryCount),
					)
				}
			}
		default:
			// Do nothing, return the error
		}

		switch decision {
		case RetryNext:
			r.retryCount++
			r.executeInternal(true)
			retried = true
		case RetrySame:
			r.retryCount++
			r.executeInternal(false)
			retried = true
		default:
			// Do nothing, return the error
		}
	}

	return retried
}

func (r *request) isBatchIdempotent(batch *codecs.PartialBatch) (idempotent bool, err error) {
	for _, query := range batch.Queries {
		switch q := query.QueryOrId.(type) {
		case string:
			if idempotent, err = parser.IsQueryIdempotent(q); !idempotent {
				return idempotent, err
			}
		case []byte:
			idempotent = r.client.proxy.isIdempotent(q)
			if !idempotent {
				return false, nil
			}
		default:
			return false, errors.New("unhandled query type in batch")
		}
	}
	return true, nil
}
