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
	"sync"

	"github.com/datastax/go-cassandra-native-protocol/frame"
)

// Request represents the data frame and lifecycle of a CQL native protocol request.
type Request interface {
	// Frame returns the frame to be executed as part of the request.
	// This must be idempotent.
	Frame() interface{}

	// Execute is called when a request need to be retried.
	// This is currently only called for executing prepared requests (i.e. `EXECUTE` request frames). If `EXECUTE`
	// request frames are not expected then the implementation should `panic()`.
	//
	// If `next` is false then the request must be retried on the current node; otherwise, it should be retried on
	// another node which is usually then next node in a query plan.
	Execute(next bool)

	// OnClose is called when the underlying connection is closed.
	// No assumptions should be made about whether the request has been successfully sent; it is possible that
	// the request has been fully sent and no response was received before
	OnClose(err error)

	// OnResult is called when a response frame has been sent back from the connection.
	OnResult(raw *frame.RawFrame)
}

type pendingRequests struct {
	pending *sync.Map
	streams chan int16
}

func newPendingRequests(maxStreams int16) *pendingRequests {
	streams := make(chan int16, maxStreams)
	for i := int16(0); i < maxStreams; i++ {
		streams <- i
	}
	return &pendingRequests{
		pending: &sync.Map{},
		streams: streams,
	}
}

func (p *pendingRequests) store(request Request) int16 {
	select {
	case stream := <-p.streams:
		p.pending.Store(stream, request)
		return stream
	default:
		return -1
	}
}

func (p *pendingRequests) loadAndDelete(stream int16) Request {
	request, ok := p.pending.LoadAndDelete(stream)
	if ok {
		p.streams <- stream
		return request.(Request)
	}
	return nil
}

func (p *pendingRequests) closing(err error) {
	p.pending.Range(func(key, value interface{}) bool {
		request := value.(Request)
		request.OnClose(err)
		return true
	})
}
