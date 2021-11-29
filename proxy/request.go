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

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
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

func (r *request) Execute(next bool) {
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
		r.Execute(true)
	} else {
		r.mu.Lock()
		if !r.done {
			r.done = true
			r.send(&message.Unavailable{ErrorMessage: "No more hosts available (cluster connection closed and request is not idempotent)"})
		}
		r.mu.Unlock()
	}
}

func (r *request) OnResult(raw *frame.RawFrame) {
	r.mu.Lock()
	if !r.done {
		r.done = true
		r.sendRaw(raw)
	}
	r.mu.Unlock()
}
