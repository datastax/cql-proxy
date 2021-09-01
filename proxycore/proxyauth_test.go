package proxycore

import (
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
)

func Test_noopProxyAuth_HandleAuthResponse(t *testing.T) {
	proxyAuth := NewNoopProxyAuth()
	assert.Equal(t, nil, proxyAuth.HandleAuthResponse(nil))
}

func Test_noopProxyAuth_MessageForStartup(t *testing.T) {
	proxyAuth := NewNoopProxyAuth()
	assert.Equal(t, &message.Ready{}, proxyAuth.MessageForStartup())
}

func Test_fakeProxyAuth_HandleAuthResponse(t *testing.T) {
	proxyAuth := NewFakeProxyAuth()
	assert.Equal(t, &message.AuthSuccess{}, proxyAuth.HandleAuthResponse(nil))
}

func Test_fakeProxyAuth_MessageForStartup(t *testing.T) {
	proxyAuth := NewFakeProxyAuth()
	assert.Equal(t, &message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"}, proxyAuth.MessageForStartup())
}