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
)

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

func (p *pendingRequests) sendError(err error) {
	p.pending.Range(func(key, value interface{}) bool {
		request := value.(Request)
		request.OnError(err)
		return true
	})
}
