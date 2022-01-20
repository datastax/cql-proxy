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
	"context"
	"sync"
	"testing"

	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
)

func TestProxy_Retries(t *testing.T) {
	const idempotentQuery = "SELECT * FROM test.test"
	const nonIdempotentQuery = "INSERT INTO test.test (k, v) VALUES ('a', uuid())"

	var tests = []struct {
		msg           string
		query         string
		response      message.Error
		numNodesTried int
		retryCount    int
	}{
		{
			"bootstrapping error",
			idempotentQuery,
			&message.IsBootstrapping{ErrorMessage: "Bootstrapping"},
			3, // Ran the query on all nodes
			2, // Retried on all remaining nodes
		},
		{
			"bootstrapping w/ non-idempotent query",
			nonIdempotentQuery,
			&message.IsBootstrapping{ErrorMessage: "Bootstrapping"},
			3, // Ran the query on all nodes
			2, // Retried on all remaining nodes
		},
		{
			"error response (truncate), retry until succeeds or exhausts query plan",
			idempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			3, // Ran the query on all nodes
			2, // Retried on all remaining nodes
		},
		{
			"error response (truncate) w/ non-idempotent query, retry until succeeds or exhausts query plan",
			nonIdempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			1, // Tried the queried on the first node and it failed
			0, // Did not retry
		},
		{
			"error response (read failure), don't retry",
			idempotentQuery,
			&message.ReadFailure{
				ErrorMessage: "",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				NumFailures:  1,
			},
			1, // Tried the query on the first node and it failed
			0, // Did not retry
		},
		{
			"error response (write failure), don't retry",
			idempotentQuery,
			&message.WriteFailure{
				ErrorMessage: "",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				NumFailures:  1,
				WriteType:    primitive.WriteTypeSimple,
			},
			1, // Tried the query on the first node and it failed
			0, // Did not retry
		},
		{
			"unavailable error, retry on the next node",
			idempotentQuery,
			&message.Unavailable{
				ErrorMessage: "Unavailable",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Required:     2,
				Alive:        1,
			},
			2, // Tried and failed on the first node, then retried on the next node
			1, // Retried on the next node
		},
		{
			"unavailable error w/ non-idempotent query, retry on the next node (same)",
			nonIdempotentQuery,
			&message.Unavailable{
				ErrorMessage: "Unavailable",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Required:     2,
				Alive:        1,
			},
			2, // Tried and failed on the first node, then retried on the next node
			1, // Retried on the next node
		},
		{
			"read timeout error, retry once on the same node",
			idempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     3,
				BlockFor:     2,
				DataPresent:  false, // Data wasn't present, read repair, retry
			},
			1, // Tried and retried on a single node
			1, // Retried on the same node
		},
		{
			"read timeout error w/ non-idempotent query, retry once on the same node (same)",
			nonIdempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     3,
				BlockFor:     2,
				DataPresent:  false, // Data wasn't present, read repair, retry
			},
			1, // Tried and retried on a single node
			1, // Retried on the same node
		},
		{
			"read timeout error w/ unmet conditions, don't retry",
			idempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				DataPresent:  true, // Data was present don't retry
			},
			1, // Tried the query on the first node and it failed
			0, // Did not retry
		},
		{
			"write timeout error, retry once if logged batch",
			// Not actually a logged batch query, but this is opaque to the proxy and mock cluster. It's considered a
			// logged batch because the error returned by the server says it is.
			idempotentQuery,
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			1, // Tried and retried on a single node
			1, // Retried on the same node
		},
		{
			"write timeout error w/ not a logged batch, don't retry",
			idempotentQuery, // Opaque idempotent query (see reason it's not an actual batch query above)
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeSimple, // Don't retry for anything other than logged batches
			},
			1, // Tried the query on the first node and it failed
			0, // Did not retry
		},
	}

	for _, tt := range tests {
		numNodesTried, retryCount, err := testProxyRetry(t, tt.query, tt.response)
		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func testProxyRetry(t *testing.T, query string, response message.Error) (numNodesTried, retryCount int, responseError error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	tried := make(map[string]int)

	cluster, proxy := setupProxyTest(t, ctx, 3, proxycore.MockRequestHandlers{
		primitive.OpCodeQuery: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
				return msg
			} else {
				mu.Lock()
				tried[cl.Local().IP]++
				mu.Unlock()
				return response
			}
		},
	})
	defer func() {
		cluster.Shutdown()
		_ = proxy.Shutdown()
	}()

	cl := connectTestClient(t, ctx)

	_, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{Query: query})

	retryCount = 0
	for _, v := range tried {
		retryCount += v
	}
	return len(tried), retryCount - 1, err
}
