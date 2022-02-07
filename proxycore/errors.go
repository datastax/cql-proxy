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
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/datastax/go-cassandra-native-protocol/message"
)

var (
	StreamsExhausted     = errors.New("streams exhausted")
	AuthExpected         = errors.New("authentication required, but no authenticator provided")
	ProtocolNotSupported = errors.New("required protocol version is not supported")
)

type UnexpectedResponse struct {
	Expected []string
	Received string
}

func (e *UnexpectedResponse) Error() string {
	return fmt.Sprintf("expected %s response(s), got %s", strings.Join(e.Expected, ", "), e.Received)
}

type CqlError struct {
	Message message.Error
}

func (e CqlError) Error() string {
	return fmt.Sprintf("cql error: %v", e.Message)
}

func isCriticalErr(err error) bool {
	// Anything that's not a temporary unavailability
	return !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, syscall.ECONNRESET)
}
