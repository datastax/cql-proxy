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
	"github.com/datastax/go-cassandra-native-protocol/message"
)

// ProxyAuthenticator is responsible for processing STARTUP from the client and preceding message/response during the auth
// handshake.
type ProxyAuthenticator interface {
	// MessageForStartup will return the proper message in response to the STARTUP request.
	MessageForStartup() message.Message
	// HandleAuthResponse will return the proper message based on implementation and the token provided by the client.
	HandleAuthResponse(token []byte) message.Message
}

// noopProxyAuth returns a READY message to the initial STARTUP request and thus will never need to handle AUTH_RESPONSE
type noopProxyAuth struct {}

func (n *noopProxyAuth) MessageForStartup() message.Message {
	return &message.Ready{}
}

func (n *noopProxyAuth) HandleAuthResponse(token []byte) message.Message {
	return nil
}

func NewNoopProxyAuth() ProxyAuthenticator {
	return &noopProxyAuth{}
}

// fakeProxyAuth imitates auth against org.apache.cassandra.auth.PasswordAuthenticator for clients that will break if they
// don't receive an AUTHENTICATE message when they expect it. Regardless of token provided will always reply with an AUTH_SUCCESS
// message.
type fakeProxyAuth struct {}

func (n *fakeProxyAuth) MessageForStartup() message.Message {
	return &message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"}
}

func (n *fakeProxyAuth) HandleAuthResponse(token []byte) message.Message {
	return &message.AuthSuccess{}
}

func NewFakeProxyAuth() ProxyAuthenticator {
	return &fakeProxyAuth{}
}