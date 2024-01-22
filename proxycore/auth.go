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
	"bytes"
	"fmt"

	"go.uber.org/zap"
)

type Authenticator interface {
	InitialResponse(authenticator string, c *ClientConn) ([]byte, error)
	EvaluateChallenge(token []byte) ([]byte, error)
	Success(token []byte) error
}

type passwordAuth struct {
	authId   string
	username string
	password string
}

const dseAuthenticator = "com.datastax.bdp.cassandra.auth.DseAuthenticator"
const passwordAuthenticator = "org.apache.cassandra.auth.PasswordAuthenticator"
const astraAuthenticator = "org.apache.cassandra.auth.AstraAuthenticator"

func (d *passwordAuth) InitialResponse(authenticator string, c *ClientConn) ([]byte, error) {
	if authenticator == dseAuthenticator {
		return []byte("PLAIN"), nil
	}
	// We'll return a SASL response but if we're seeing an authenticator we're unfamiliar with at least log
	// that information here
	if (authenticator != passwordAuthenticator) && (authenticator != astraAuthenticator) {
		c.logger.Info("observed unknown authenticator, treating as SASL",
			zap.String("authenticator", authenticator))
	}
	return d.makeToken(), nil
}

func (d *passwordAuth) EvaluateChallenge(token []byte) ([]byte, error) {
	if token == nil || bytes.Compare(token, []byte("PLAIN-START")) != 0 {
		return nil, fmt.Errorf("incorrect SASL challenge from server, expecting PLAIN-START, got: %v", string(token))
	}
	return d.makeToken(), nil
}

func (d *passwordAuth) makeToken() []byte {
	token := bytes.NewBuffer(make([]byte, 0, len(d.authId)+len(d.username)+len(d.password)+2))
	token.WriteString(d.authId)
	token.WriteByte(0)
	token.WriteString(d.username)
	token.WriteByte(0)
	token.WriteString(d.password)
	return token.Bytes()
}

func (d *passwordAuth) Success(_ []byte) error {
	return nil
}

func NewPasswordAuth(username string, password string) Authenticator {
	return &passwordAuth{
		authId:   "",
		username: username,
		password: password,
	}
}
