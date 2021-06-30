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

func TestLookupEndpointInvalid(t *testing.T) {
	var tests = []struct {
		addr string
		err  string
	}{
		{"localhost", "missing port in address"},
		{"dne:1234", "no such host"},
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
