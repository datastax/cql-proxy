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
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

type testRequest struct {
	stream int16
	errs   *[]error
}

func (t testRequest) Frame() interface{} {
	panic("implement me")
}

func (t *testRequest) OnClose(err error) {
	*t.errs = append(*t.errs, err)
}

func (t testRequest) OnResult(_ *frame.RawFrame) {
	panic("implement me")
}

func TestPendingRequests(t *testing.T) {
	const max = 10

	p := newPendingRequests(max)

	errs := make([]error, 0)

	for i := int16(0); i < max; i++ {
		assert.Equal(t, i, p.store(&testRequest{stream: i, errs: &errs}))
	}
	assert.Equal(t, int16(-1), p.store(&testRequest{}))

	r := p.loadAndDelete(0).(*testRequest)
	assert.Equal(t, int16(0), r.stream)

	r = p.loadAndDelete(9).(*testRequest)
	assert.Equal(t, int16(9), r.stream)

	assert.Equal(t, int16(0), p.store(&testRequest{stream: 0, errs: &errs}))
	assert.Equal(t, int16(9), p.store(&testRequest{stream: 9, errs: &errs}))
	assert.Equal(t, int16(-1), p.store(&testRequest{}))

	p.closing(io.EOF)

	assert.Equal(t, 10, len(errs))

	for _, err := range errs {
		assert.ErrorAs(t, err, &io.EOF)
	}
}
