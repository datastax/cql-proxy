// Copyright 2020 DataStax
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
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type CqlError interface {
	error
	Message() *message.Error
}

func NewCqlError(msg *message.Error) CqlError {
	return &defaultCqlError{
		msg,
	}
}

type defaultCqlError struct {
	msg *message.Error
}

func (d *defaultCqlError) Error() string {
	return fmt.Sprintf("cql error: %v", d.msg)
}

func (d *defaultCqlError) Message() *message.Error {
	return d.msg
}
