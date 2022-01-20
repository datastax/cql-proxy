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

package proxy

import (
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryPolicy_OnUnavailable(t *testing.T) {
	var tests = []struct {
		msg        *message.Unavailable
		decision   RetryDecision
		retryCount int
	}{
		{&message.Unavailable{Consistency: 0, Required: 0, Alive: 0}, RetryNext, 0},   // Never retried
		{&message.Unavailable{Consistency: 0, Required: 0, Alive: 0}, ReturnError, 1}, // Already retried once
	}

	policy := NewDefaultRetryPolicy()
	for _, tt := range tests {
		decision := policy.OnUnavailable(tt.msg, tt.retryCount)
		assert.Equal(t, tt.decision, decision)
	}
}

func TestDefaultRetryPolicy_OnReadTimeout(t *testing.T) {
	var tests = []struct {
		msg        *message.ReadTimeout
		decision   RetryDecision
		retryCount int
	}{
		{&message.ReadTimeout{Consistency: 0, Received: 2, BlockFor: 2, DataPresent: false}, RetrySame, 0},   // Enough received with no data
		{&message.ReadTimeout{Consistency: 0, Received: 3, BlockFor: 2, DataPresent: false}, ReturnError, 1}, // Already retried once
		{&message.ReadTimeout{Consistency: 0, Received: 2, BlockFor: 2, DataPresent: true}, ReturnError, 0},  // Data was present
	}

	policy := NewDefaultRetryPolicy()
	for _, tt := range tests {
		decision := policy.OnReadTimeout(tt.msg, tt.retryCount)
		assert.Equal(t, tt.decision, decision)
	}
}

func TestDefaultRetryPolicy_OnWriteTimeout(t *testing.T) {
	var tests = []struct {
		msg        *message.WriteTimeout
		decision   RetryDecision
		retryCount int
	}{
		{&message.WriteTimeout{Consistency: 0, Received: 0, BlockFor: 0, WriteType: primitive.WriteTypeBatchLog, Contentions: 0}, RetrySame, 0},   // Logged batch
		{&message.WriteTimeout{Consistency: 0, Received: 0, BlockFor: 0, WriteType: primitive.WriteTypeBatchLog, Contentions: 0}, ReturnError, 1}, // Logged batch, already retried once
		{&message.WriteTimeout{Consistency: 0, Received: 0, BlockFor: 0, WriteType: primitive.WriteTypeSimple, Contentions: 0}, ReturnError, 0},   // Not a logged batch
	}

	policy := NewDefaultRetryPolicy()
	for _, tt := range tests {
		decision := policy.OnWriteTimeout(tt.msg, tt.retryCount)
		assert.Equal(t, tt.decision, decision)
	}
}

func TestDefaultRetryPolicy_OnErrorResponse(t *testing.T) {
	var tests = []struct {
		msg        message.Error
		decision   RetryDecision
		retryCount int
	}{
		{&message.WriteFailure{}, ReturnError, 0}, // Write failure
		{&message.ReadFailure{}, ReturnError, 0},  // Read failure
		{&message.TruncateError{}, RetryNext, 0},  // Truncate failure
		{&message.ServerError{}, RetryNext, 0},    // Server failure
		{&message.Overloaded{}, RetryNext, 0},     // Overloaded failure
	}

	policy := NewDefaultRetryPolicy()
	for _, tt := range tests {
		decision := policy.OnErrorResponse(tt.msg, tt.retryCount)
		assert.Equal(t, tt.decision, decision)
	}
}
