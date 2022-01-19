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
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// RetryDecision is a type used for deciding what to do when a request has failed.
type RetryDecision int

func (r RetryDecision) String() string {
	switch r {
	case RetrySame:
		return "retry same node"
	case RetryNext:
		return "retry next node"
	case ReturnError:
		return "returning error"
	}
	return "unknown"
}

const (
	// RetrySame should be returned when a request should be retried on the same host.
	RetrySame RetryDecision = iota
	// RetryNext should be returned when a request should be retried on the next host according to the request's query
	// plan.
	RetryNext
	// ReturnError should be returned when a request's original error should be forwarded along to the client.
	ReturnError
)

// RetryPolicy is an interface for defining retry behavior when a server-side error occurs.
type RetryPolicy interface {
	// OnReadTimeout handles the retry decision for a server-side read timeout error (Read_timeout = 0x1200).
	// This occurs when a replica read request times out during a read query.
	OnReadTimeout(msg *message.ReadTimeout, retryCount int) RetryDecision

	// OnWriteTimeout handles the retry decision for a server-side write timeout error (Write_timeout = 0x1100).
	// This occurs when a replica write request times out during a write query.
	OnWriteTimeout(msg *message.WriteTimeout, retryCount int) RetryDecision

	// OnUnavailable handles the retry decision for a server-side unavailable exception (Unavailable = 0x1000).
	// This occurs when a coordinator determines that there are not enough replicas to handle a query at the requested
	// consistency level.
	OnUnavailable(msg *message.Unavailable, retryCount int) RetryDecision

	// OnErrorResponse handles the retry decision for other potentially recoverable errors.
	// This can be called for the following error types: server error (ServerError = 0x0000),
	// overloaded (Overloaded = 0x1001), truncate error (Truncate_error = 0x1003), read failure (Read_failure = 0x1300),
	// and write failure (Write_failure = 0x1500).
	OnErrorResponse(msg message.Error, retryCount int) RetryDecision
}

type defaultRetryPolicy struct{}

var defaultRetryPolicyInstance defaultRetryPolicy

// NewDefaultRetryPolicy creates a new default retry policy.
// The default retry policy takes a conservative approach to retrying requests. In most cases it retries only once in
// cases where a retry is likely to succeed.
func NewDefaultRetryPolicy() RetryPolicy {
	return &defaultRetryPolicyInstance
}

// OnReadTimeout retries in the case where there were enough replicas to satisfy the request, but one of the replicas
// didn't respond with data and timed out. It's likely that a single retry to the same coordinator will succeed because
// it will have recognized the replica as dead before the retry is attempted.
//
// In all other cases it will forward the original error to the client.
func (d defaultRetryPolicy) OnReadTimeout(msg *message.ReadTimeout, retryCount int) RetryDecision {
	if retryCount == 0 && msg.Received >= msg.BlockFor && !msg.DataPresent {
		return RetrySame
	} else {
		return ReturnError
	}
}

// OnWriteTimeout retries in the case where a coordinator failed to write its batch log to a set of datacenter local
// nodes. It's likely that a single retry to the same coordinator will succeed because it will have recognized the
// dead nodes and use a different set of nodes.
//
// In all other cases it will forward the original error to the client.
func (d defaultRetryPolicy) OnWriteTimeout(msg *message.WriteTimeout, retryCount int) RetryDecision {
	if retryCount == 0 && msg.WriteType == primitive.WriteTypeBatchLog {
		return RetrySame
	} else {
		return ReturnError
	}
}

// OnUnavailable retries, once, on the next coordinator in the query plan. This is to handle the case where a
// coordinator is failing because it was partitioned from a set of its replicas.
func (d defaultRetryPolicy) OnUnavailable(_ *message.Unavailable, retryCount int) RetryDecision {
	if retryCount == 0 {
		return RetryNext
	} else {
		return ReturnError
	}
}

// OnErrorResponse retries on the next coordinator for all error types except for read and write failures.
func (d defaultRetryPolicy) OnErrorResponse(msg message.Error, retryCount int) RetryDecision {
	code := msg.GetErrorCode()
	if code == primitive.ErrorCodeReadFailure || code == primitive.ErrorCodeWriteFailure {
		return ReturnError
	} else {
		return RetryNext
	}
}
