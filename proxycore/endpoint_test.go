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
	"crypto/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLookupEndpoint(t *testing.T) {
	addr, err := LookupEndpoint(&testEndpoint{addr: "localhost:9042"})
	require.NoError(t, err, "unable to lookup endpoint")
	assert.Equal(t, "127.0.0.1:9042", addr)

	addr, err = LookupEndpoint(&testEndpoint{addr: "127.0.0.1:9042", isResolved: true})
	require.NoError(t, err, "unable to lookup endpoint")
	assert.Equal(t, "127.0.0.1:9042", addr)
}

func TestLookupEndpoint_Invalid(t *testing.T) {
	var tests = []struct {
		addr string
		err  string
	}{
		{"localhost", "missing port in address"},
		{"dne:1234", ""}, // Errors for DNS can vary per system
	}

	for _, tt := range tests {
		_, err := LookupEndpoint(&testEndpoint{addr: tt.addr})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), tt.err)
		}
	}
}

type testEndpoint struct {
	addr       string
	isResolved bool
}

func (t testEndpoint) String() string {
	return t.addr
}

func (t testEndpoint) Addr() string {
	return t.addr
}

func (t testEndpoint) IsResolved() bool {
	return t.isResolved
}

func (t testEndpoint) TlsConfig() *tls.Config {
	return nil
}

func (t testEndpoint) Key() string {
	return t.addr
}
