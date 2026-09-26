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
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"sync"

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
type noopProxyAuth struct{}

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
type fakeProxyAuth struct{}

func (n *fakeProxyAuth) MessageForStartup() message.Message {
	return &message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"}
}

func (n *fakeProxyAuth) HandleAuthResponse(token []byte) message.Message {
	return &message.AuthSuccess{}
}

func NewFakeProxyAuth() ProxyAuthenticator {
	return &fakeProxyAuth{}
}

// CredentialStore stores and validates username/password pairs
type CredentialStore struct {
	users map[string]string // username -> password hash
	mu    sync.RWMutex
}

func NewCredentialStore() *CredentialStore {
	return &CredentialStore{
		users: make(map[string]string),
	}
}

func (cs *CredentialStore) AddUser(username, password string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	// Simple hash for demo - use bcrypt or similar in production
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	cs.users[username] = hash
}

func (cs *CredentialStore) Validate(username, password string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	storedHash, exists := cs.users[username]
	if !exists {
		return false
	}

	providedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	return storedHash == providedHash
}

func (cs *CredentialStore) LoadFromEnv() {
	// Load credentials from environment variables
	// Format: USERNAME1=password1,USERNAME2=password2
	creds := os.Getenv("CQL_CREDENTIALS")
	if creds == "" {
		return
	}

	pairs := strings.Split(creds, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			cs.AddUser(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func (cs *CredentialStore) UserCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.users)
}

// passwordProxyAuth implements real password authentication that validates credentials
type passwordProxyAuth struct {
	credStore *CredentialStore
}

func (p *passwordProxyAuth) MessageForStartup() message.Message {
	return &message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"}
}

func (p *passwordProxyAuth) HandleAuthResponse(token []byte) message.Message {
	// Parse PasswordAuthenticator token format: \x00<username>\x00<password>
	if len(token) < 2 || token[0] != 0 {
		return &message.AuthenticationError{ErrorMessage: "Invalid authentication token format"}
	}

	// Find null separator
	nullPos := -1
	for i := 1; i < len(token); i++ {
		if token[i] == 0 {
			nullPos = i
			break
		}
	}

	if nullPos == -1 {
		return &message.AuthenticationError{ErrorMessage: "Invalid authentication token format: no separator"}
	}

	username := string(token[1:nullPos])
	password := string(token[nullPos+1:])

	// Validate credentials
	if !p.credStore.Validate(username, password) {
		return &message.AuthenticationError{ErrorMessage: "Invalid username or password"}
	}

	return &message.AuthSuccess{}
}

// NewPasswordProxyAuth creates a new password authenticator that validates credentials
// Credentials should be loaded via CredentialStore.LoadFromEnv() or manually added
func NewPasswordProxyAuth(credStore *CredentialStore) ProxyAuthenticator {
	return &passwordProxyAuth{
		credStore: credStore,
	}
}
