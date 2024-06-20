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
	"crypto/md5"
	"sync"
	"testing"

	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const idempotentQuery = "INSERT INTO test.test (k, v) VALUES ('a', 123e4567-e89b-12d3-a456-426614174000)"
const nonIdempotentQuery = "INSERT INTO test.test (k, v) VALUES ('a', uuid())"

var idempotentQueryHash = md5.Sum([]byte(idempotentQuery))
var nonIdempotentQueryHash = md5.Sum([]byte(nonIdempotentQuery))

var preparedStmts = map[[16]byte]string{
	idempotentQueryHash:    idempotentQuery,
	nonIdempotentQueryHash: nonIdempotentQuery,
}

func TestProxy_Retries(t *testing.T) {
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
			"error response (truncate) w/ non-idempotent query, don't retry",
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
		numNodesTried, retryCount, err := testProxyRetry(t, &message.Query{Query: tt.query}, tt.response, tt.msg)
		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func TestProxy_PreparedRetries(t *testing.T) {
	var tests = []struct {
		msg           string
		execute       *message.Execute
		response      message.Error
		numNodesTried int
		retryCount    int
	}{
		{
			"idempotent prepared query, retry on all nodes",
			&message.Execute{QueryId: idempotentQueryHash[:]},
			&message.ServerError{ErrorMessage: "some server error"},
			3,
			2,
		},
		{
			"non-idempotent prepared query, don't retry",
			&message.Execute{QueryId: nonIdempotentQueryHash[:]},
			&message.ServerError{ErrorMessage: "some server error"},
			1,
			0,
		},
	}

	for _, tt := range tests {
		numNodesTried, retryCount, err := testProxyRetry(t, tt.execute, tt.response, tt.msg)
		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func TestProxy_BatchRetries(t *testing.T) {
	var tests = []struct {
		msg           string
		batch         *message.Batch
		response      message.Error
		numNodesTried int
		retryCount    int
	}{
		{
			"write timeout error, retry once if logged batch",
			&message.Batch{Children: []*message.BatchChild{
				{Query: idempotentQuery},
			}},
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			1,
			1,
		},
		{
			"write timeout error, retry once if logged batch w/ prepared statement",
			&message.Batch{Children: []*message.BatchChild{
				{Id: idempotentQueryHash[:]},
			}},
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			1,
			1,
		},
		{
			"batch w/ non-idempotent query, don't retry",
			&message.Batch{Children: []*message.BatchChild{
				{Query: nonIdempotentQuery},
			}},
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			1,
			0,
		},
		{
			"batch w/ non-idempotent prepared query, don't retry",
			&message.Batch{Children: []*message.BatchChild{
				{Id: nonIdempotentQueryHash[:]},
			}},
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			1,
			0,
		},
	}

	for _, tt := range tests {
		numNodesTried, retryCount, err := testProxyRetry(t, tt.batch, tt.response, tt.msg)
		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func TestProxy_RetryGraphQueries(t *testing.T) {
	var tests = []struct {
		msg           string
		query         string
		graph         bool
		cfg           *proxyTestConfig
		response      message.Error
		numNodesTried int
		retryCount    int
	}{
		{
			"error response (truncate) w/ graph query, not retried",
			"g.V().has('person', 'name', 'mike')",
			true,
			nil,
			&message.TruncateError{ErrorMessage: "Truncate"},
			1, // Tried on the first node and fails
			0, // Not retried because graph queries are not considered idempotent
		},
		{
			"error response (truncate) w/ graph query and idempotent override; retried on all nodes",
			"g.V().has('person', 'name', 'mike')",
			true,
			&proxyTestConfig{idempotentGraph: true}, // Override to consider graph queries as idempotent
			&message.TruncateError{ErrorMessage: "Truncate"},
			3, // Tried on all nodes because of the idempotent override
			2, // Retried twice after the initial failure
		},
		{
			"error response (truncate) w/ non-idempotent, non-graph query, should *not* be retried",
			nonIdempotentQuery,
			false,
			&proxyTestConfig{idempotentGraph: true}, // Override to consider graph queries as idempotent, but not CQL
			&message.TruncateError{ErrorMessage: "Truncate"},
			1, // Tried on the first node and fails
			0, // Not retried because graph queries are not considered idempotent
		},
	}

	for _, tt := range tests {
		frm := frame.NewFrame(primitive.ProtocolVersion4, -1, &message.Query{Query: tt.query})
		if tt.graph {
			frm.SetCustomPayload(map[string][]byte{"graph-source": []byte("g")}) // This is used by the proxy to determine if it's a graph query
		}

		numNodesTried, retryCount, err := testProxyRetryWithConfig(t, frm, tt.response, tt.cfg, tt.msg)

		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func testProxyRetryWithConfig(t *testing.T, query *frame.Frame, response message.Error, cfg *proxyTestConfig, testMessage string) (numNodesTried, retryCount int, responseError error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	tried := make(map[string]int)
	prepared := make(map[[16]byte]string)

	if cfg == nil {
		cfg = &proxyTestConfig{}
	}

	cfg.handlers = proxycore.MockRequestHandlers{
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
		primitive.OpCodeExecute: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			msg := frm.Body.Message.(*message.Execute)
			mu.Lock()
			defer mu.Unlock()
			var id [16]byte
			copy(id[:], msg.QueryId)
			if _, ok := prepared[id]; !ok {
				return &message.Unprepared{
					ErrorMessage: "query is not prepared",
					Id:           id[:],
				}
			} else {
				tried[cl.Local().IP]++
				return response
			}
		},
		primitive.OpCodeBatch: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			msg := frm.Body.Message.(*message.Batch)
			mu.Lock()
			defer mu.Unlock()
			for _, child := range msg.Children {
				id := child.Id
				if id != nil {
					var hash [16]byte
					copy(hash[:], id)
					if _, ok := prepared[hash]; !ok {
						return &message.Unprepared{
							ErrorMessage: "query is not prepared",
							Id:           id,
						}
					}
				}
			}
			tried[cl.Local().IP]++
			return response
		},
		primitive.OpCodePrepare: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			msg := frm.Body.Message.(*message.Prepare)
			mu.Lock()
			defer mu.Unlock()
			id := md5.Sum([]byte(msg.Query))
			prepared[id] = msg.Query
			return &message.PreparedResult{
				PreparedQueryId: id[:],
			}
		},
	}

	tester, proxyContactPoint, err := setupProxyTestWithConfig(ctx, 3, cfg)
	defer func() {
		cancel()
		tester.shutdown()
	}()
	require.NoError(t, err, testMessage)

	cl := connectTestClient(t, ctx, proxyContactPoint)

	_, err = cl.QueryFrame(ctx, query)

	if cqlErr, ok := err.(*proxycore.CqlError); ok {
		if unprepared, ok := cqlErr.Message.(*message.Unprepared); ok {
			var hash [16]byte
			copy(hash[:], unprepared.Id)
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Prepare{Query: preparedStmts[hash]})
			if err != nil {
				return 0, 0, err
			}
			_, err = cl.QueryFrame(ctx, query)
		}
	}

	retryCount = 0
	for _, v := range tried {
		retryCount += v
	}
	return len(tried), retryCount - 1, err
}

func testProxyRetry(t *testing.T, query message.Message, response message.Error, testMessage string) (numNodesTried, retryCount int, responseError error) {
	return testProxyRetryWithConfig(t, frame.NewFrame(primitive.ProtocolVersion4, -1, query), response, nil, testMessage)
}
