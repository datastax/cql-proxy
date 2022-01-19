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
			3,
			2,
		},
		{
			"bootstrapping w/ non-idempotent query",
			nonIdempotentQuery,
			&message.IsBootstrapping{ErrorMessage: "Bootstrapping"},
			3,
			2,
		},
		{
			"error response (truncate), retry until succeeds or exhausts query plan",
			idempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			3,
			2,
		},
		{
			"error response (truncate) w/ non-idempotent query, retry until succeeds or exhausts query plan",
			nonIdempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			1,
			0,
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
			1,
			0,
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
			1,
			0,
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
			2,
			1,
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
			2,
			1,
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
			1,
			1,
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
			1,
			1,
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
			1,
			0,
		},
		{
			"write timeout error, retry once if logged batch",
			idempotentQuery, // Not a logged batch, but it doesn't matter for this test
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
			"write timeout error w/ unmet conditions, don't retry",
			idempotentQuery,
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeSimple, // Don't retry for anything other than logged batches
			},
			1,
			0,
		},
	}

	for _, tt := range tests {
		numNodesTried, retryCount, err := testProxyRetry(t, &message.Query{Query: tt.query}, tt.response)
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
		numNodesTried, retryCount, err := testProxyRetry(t, tt.execute, tt.response)
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
				{QueryOrId: idempotentQuery},
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
				{QueryOrId: idempotentQueryHash[:]},
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
				{QueryOrId: nonIdempotentQuery},
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
				{QueryOrId: nonIdempotentQueryHash[:]},
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
		numNodesTried, retryCount, err := testProxyRetry(t, tt.batch, tt.response)
		assert.Error(t, err, tt.msg)
		assert.IsType(t, err, &proxycore.CqlError{}, tt.msg)
		assert.Equal(t, tt.numNodesTried, numNodesTried, tt.msg)
		assert.Equal(t, tt.retryCount, retryCount, tt.msg)
	}
}

func testProxyRetry(t *testing.T, query message.Message, response message.Error) (numNodesTried, retryCount int, responseError error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	tried := make(map[string]int)
	prepared := make(map[[16]byte]string)

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
				if id, ok := child.QueryOrId.([]byte); ok {
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
	})
	defer func() {
		cluster.Shutdown()
		_ = proxy.Shutdown()
	}()

	cl := connectTestClient(t, ctx)

	_, err := cl.Query(ctx, primitive.ProtocolVersion4, query)

	if cqlErr, ok := err.(*proxycore.CqlError); ok {
		if unprepared, ok := cqlErr.Message.(*message.Unprepared); ok {
			var hash [16]byte
			copy(hash[:], unprepared.Id)
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Prepare{Query: preparedStmts[hash]})
			if err != nil {
				return 0, 0, err
			}
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, query)
		}
	}

	retryCount = 0
	for _, v := range tried {
		retryCount += v
	}
	return len(tried), retryCount - 1, err
}
