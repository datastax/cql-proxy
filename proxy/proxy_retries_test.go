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
		query         string
		response      message.Error
		errString     string
		numNodesTried int
		retryCount    int
	}{
		{ // Bootstrapping error
			idempotentQuery,
			&message.IsBootstrapping{ErrorMessage: "Bootstrapping"},
			"cql error: ERROR UNAVAILABLE (code=ErrorCode Unavailable [0x00001000], msg=No more hosts available (exhausted query plan), cl=ConsistencyLevel ANY [0x0000], required=0, alive=0)",
			3,
			2,
		},
		{ // Bootstrapping error w/ non-idempotent query
			nonIdempotentQuery,
			&message.IsBootstrapping{ErrorMessage: "Bootstrapping"},
			"cql error: ERROR UNAVAILABLE (code=ErrorCode Unavailable [0x00001000], msg=No more hosts available (exhausted query plan), cl=ConsistencyLevel ANY [0x0000], required=0, alive=0)",
			3,
			2,
		},
		{ // Error response (truncate), retry until succeeds or exhausts query plan
			idempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			"cql error: ERROR UNAVAILABLE (code=ErrorCode Unavailable [0x00001000], msg=No more hosts available (exhausted query plan), cl=ConsistencyLevel ANY [0x0000], required=0, alive=0)",
			3,
			2,
		},
		{ // Error response (truncate) w/ non-idempotent query, retry until succeeds or exhausts query plan
			nonIdempotentQuery,
			&message.TruncateError{ErrorMessage: "Truncate"},
			"cql error: ERROR TRUNCATE ERROR (code=ErrorCode TruncateError [0x00001003], msg=Truncate)",
			1,
			0,
		},
		{ // Error response (read failure), don't retry
			idempotentQuery,
			&message.ReadFailure{
				ErrorMessage: "",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				NumFailures:  1,
			},
			"cql error: ERROR READ FAILURE (code=ErrorCode ReadFailure [0x00001300], msg=, cl=ConsistencyLevel QUORUM [0x0004], received=2, blockfor=2, data=false)",
			1,
			0,
		},
		{ // Error response (write failure), don't retry
			idempotentQuery,
			&message.WriteFailure{
				ErrorMessage: "",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				NumFailures:  1,
				WriteType:    primitive.WriteTypeSimple,
			},
			"cql error: ERROR WRITE FAILURE (code=ErrorCode WriteFailure [0x00001500], msg=, cl=ConsistencyLevel QUORUM [0x0004], received=2, blockfor=2, type=SIMPLE)",
			1,
			0,
		},
		{ // Unavailable error, retry on the next node
			idempotentQuery,
			&message.Unavailable{
				ErrorMessage: "Unavailable",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Required:     2,
				Alive:        1,
			},
			"cql error: ERROR UNAVAILABLE (code=ErrorCode Unavailable [0x00001000], msg=Unavailable, cl=ConsistencyLevel QUORUM [0x0004], required=2, alive=1)",
			2,
			1,
		},
		{ // Unavailable error w/ non-idempotent query, retry on the next node (same)
			nonIdempotentQuery,
			&message.Unavailable{
				ErrorMessage: "Unavailable",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Required:     2,
				Alive:        1,
			},
			"cql error: ERROR UNAVAILABLE (code=ErrorCode Unavailable [0x00001000], msg=Unavailable, cl=ConsistencyLevel QUORUM [0x0004], required=2, alive=1)",
			2,
			1,
		},
		{ // Read timeout error, retry once on the same node
			idempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     3,
				BlockFor:     2,
				DataPresent:  false, // Data wasn't present, read repair, retry
			},
			"cql error: ERROR READ TIMEOUT (code=ErrorCode ReadTimeout [0x00001200], msg=ReadTimeout, cl=ConsistencyLevel QUORUM [0x0004], received=3, blockfor=2, data=false)",
			1,
			1,
		},
		{ // Read timeout error w/ non-idempotent query, retry once on the same node (same)
			nonIdempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     3,
				BlockFor:     2,
				DataPresent:  false, // Data wasn't present, read repair, retry
			},
			"cql error: ERROR READ TIMEOUT (code=ErrorCode ReadTimeout [0x00001200], msg=ReadTimeout, cl=ConsistencyLevel QUORUM [0x0004], received=3, blockfor=2, data=false)",
			1,
			1,
		},
		{ // Read timeout error w/ unmet conditions, don't retry
			idempotentQuery,
			&message.ReadTimeout{
				ErrorMessage: "ReadTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     2,
				BlockFor:     2,
				DataPresent:  true, // Data was present don't retry
			},
			"cql error: ERROR READ TIMEOUT (code=ErrorCode ReadTimeout [0x00001200], msg=ReadTimeout, cl=ConsistencyLevel QUORUM [0x0004], received=2, blockfor=2, data=true)",
			1,
			0,
		},
		{ // Write timeout error, retry once if logged batch
			idempotentQuery, // Not a logged batch, but it doesn't matter for this test
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeBatchLog, // Retry if a logged batch
			},
			"cql error: ERROR WRITE TIMEOUT (code=ErrorCode WriteTimeout [0x00001100], msg=WriteTimeout, cl=ConsistencyLevel QUORUM [0x0004], received=1, blockfor=2, type=BATCH_LOG, contentions=0)",
			1,
			1,
		},
		{ // Write timeout error w/ unmet conditions, don't retry
			idempotentQuery,
			&message.WriteTimeout{
				ErrorMessage: "WriteTimeout",
				Consistency:  primitive.ConsistencyLevelQuorum,
				Received:     1,
				BlockFor:     2,
				WriteType:    primitive.WriteTypeSimple, // Don't retry for anything other than logged batches
			},
			"cql error: ERROR WRITE TIMEOUT (code=ErrorCode WriteTimeout [0x00001100], msg=WriteTimeout, cl=ConsistencyLevel QUORUM [0x0004], received=1, blockfor=2, type=SIMPLE, contentions=0)",
			1,
			0,
		},
	}

	for _, tt := range tests {
		numNodesTried, retryCount, err := testProxyRetry(t, tt.query, tt.response)
		assert.EqualError(t, err, tt.errString)
		assert.Equal(t, tt.numNodesTried, numNodesTried)
		assert.Equal(t, tt.retryCount, retryCount)
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
