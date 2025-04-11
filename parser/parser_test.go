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

package parser

import (
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/datacodec"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	var tests = []struct {
		keyspace   string
		query      string
		handled    bool
		idempotent bool
		stmt       Statement
		hasError   bool
	}{
		{"", "SELECT key, rpc_address AS address, count(*) FROM system.local", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "local",
			Selectors: []Selector{
				&IDSelector{Name: "key"},
				&AliasSelector{Alias: "address", Selector: &IDSelector{Name: "rpc_address"}},
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"system", "SELECT count(*) FROM local", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "local",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"system", "SELECT count(*) FROM \"local\"", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "local",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"", "SELECT count(*) FROM system.peers", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "peers",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"", "SELECT count(*) FROM \"system\".\"peers\"", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "peers",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"system", "SELECT count(*) FROM peers", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "peers",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"", "SELECT count(*) FROM system.peers_v2", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "peers_v2",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"system", "SELECT count(*) FROM peers_v2", true, true, &SelectStatement{
			Keyspace: "system",
			Table:    "peers_v2",
			Selectors: []Selector{
				&CountFuncSelector{Arg: "*"},
			},
		}, false},
		{"", "SELECT func(key) FROM system.local", true, true, nil, true},
		{"", "SELECT JSON * FROM system.local", true, true, nil, true},
		{"", "SELECT DISTINCT * FROM system.local", true, true, nil, true},
		{"", "USE system", true, false, &UseStatement{
			Keyspace: "system",
		}, false},
		// Reads from tables named similarly to system tables (not handled)
		{"", "SELECT count(*) FROM local", false, true, defaultSelectStatement, false},
		{"", "SELECT count(*) FROM peers", false, true, defaultSelectStatement, false},
		{"", "SELECT count(*) FROM peers_v2", false, true, defaultSelectStatement, false},

		// Semicolon at the end
		{"", "SELECT count(*) FROM table;", false, true, defaultSelectStatement, false},

		// Mutations to system tables (not handled)
		{"", "INSERT INTO system.local (key, rpc_address) VALUES ('local1', '127.0.0.1')", false, true, nil, false},
		{"", "UPDATE system.local SET rpc_address = '127.0.0.1' WHERE key = 'local'", false, true, nil, false},
		{"", "DELETE rpc_address FROM system.local WHERE key = 'local'", false, true, nil, false},

		// Mutations that use the functions whose values change e.g. uuid(), now() (not idempotent)
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', uuid())", false, false, nil, false},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', now())", false, false, nil, false},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', nested(now()))", false, false, nil, false},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', nested(uuid()))", false, false, nil, false},
		{"", "UPDATE ks.table SET v = uuid() WHERE k = 1", false, false, nil, false},
		{"", "UPDATE ks.table SET v = now() WHERE k = 1", false, false, nil, false},
		{"", "UPDATE ks.table SET v = nested(uuid()) WHERE k = 1", false, false, nil, false},
		{"", "UPDATE ks.table SET v = nested(now()) WHERE k = 1", false, false, nil, false},

		// Updates that prepend/append to a list (not idempotent)
		{"", "UPDATE ks.table SET v = v + [1] WHERE k = 1", false, false, nil, false}, // Append
		{"", "UPDATE ks.table SET v = [1] + v WHERE k = 1", false, false, nil, false}, // Prepend
		{"", "UPDATE ks.table SET v += [1] WHERE k = 1", false, false, nil, false},    // Append assign
		{"", "UPDATE ks.table SET v -= [1] WHERE k = 1", false, false, nil, false},    // Remove assign

		// Updates to counter values (not idempotent)
		{"", "UPDATE ks.table SET v = v + 1 WHERE k = 1", false, false, nil, false}, // Left add
		{"", "UPDATE ks.table SET v = 1 + v WHERE k = 1", false, false, nil, false}, // Right add
		{"", "UPDATE ks.table SET v += 1 WHERE k = 1", false, false, nil, false},    // Add assign
		{"", "UPDATE ks.table SET v = v - 1 WHERE k = 1", false, false, nil, false}, // Left subtract
		{"", "UPDATE ks.table SET v -= 1 WHERE k = 1", false, false, nil, false},    // Subtract assign

		// Update set/map (idempotent)
		{"", "UPDATE ks.table SET v = v + { 1 } WHERE k = 1", false, true, nil, false},        // Add to set (right)
		{"", "UPDATE ks.table SET v = { 1 } + v WHERE k = 1", false, true, nil, false},        // Add to set (left)
		{"", "UPDATE ks.table SET v = v + { 'a': 1 } WHERE k = 1", false, true, nil, false},   // Add to map (right)
		{"", "UPDATE ks.table SET v =  { 'a': 1 } +  v WHERE k = 1", false, true, nil, false}, // Add to map (left)

		// Deletes to elements of a collection (not idempotent)
		{"", "DELETE v[0] FROM ks.table WHERE k = 1", false, false, nil, false},

		// Deletes to elements of a collection (idempotent)
		{"", "DELETE v['a'] FROM ks.table WHERE k = 1", false, true, nil, false},                                  // String
		{"", "DELETE v[0.0] FROM ks.table WHERE k = 1", false, true, nil, false},                                  // Float
		{"", "DELETE v[0x1] FROM ks.table WHERE k = 1", false, true, nil, false},                                  // Hex (which is different from int)
		{"", "DELETE v[true] FROM ks.table WHERE k = 1", false, true, nil, false},                                 // Boolean true
		{"", "DELETE v[false] FROM ks.table WHERE k = 1", false, true, nil, false},                                // Boolean false
		{"", "DELETE v[(1,2,3)] FROM ks.table WHERE k = 1", false, true, nil, false},                              // Tuple
		{"", "DELETE v[{field1: 1, field2: 'a'}] FROM ks.table WHERE k = 1", false, true, nil, false},             // UDT
		{"", "DELETE v[[1,2,3]] FROM ks.table WHERE k = 1", false, true, nil, false},                              // List
		{"", "DELETE v[{1,2,3}] FROM ks.table WHERE k = 1", false, true, nil, false},                              // Set
		{"", "DELETE v[{'a': 1}] FROM ks.table WHERE k = 1", false, true, nil, false},                             // Map
		{"", "DELETE v[Nan] FROM ks.table WHERE k = 1", false, true, nil, false},                                  // Nan (float)
		{"", "DELETE v[Infinity] FROM ks.table WHERE k = 1", false, true, nil, false},                             // Infinity (float)
		{"", "DELETE v[null] FROM ks.table WHERE k = 1", false, true, nil, false},                                 // Null
		{"", "DELETE v[2021Y12M03D] FROM ks.table WHERE k = 1", false, true, nil, false},                          // Duration
		{"", "DELETE v[123e4567-e89b-12d3-a456-426614174000] FROM ks.table WHERE k = 1", false, true, nil, false}, // UUID

		// Lightweight transactions (LWTs are not idempotent)
		{"", "INSERT INTO ks.table (k, v) VALUES ('a', 1) IF NOT EXISTS", false, false, nil, false},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF v > 2", false, false, nil, false},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF EXISTS", false, false, nil, false},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF NOT EXISTS", false, false, nil, false},
		{"", "DELETE FROM ks.table WHERE k = 'a' IF EXISTS", false, false, nil, false},
		{"", "DELETE a.b, c.d FROM ks.table WHERE k = 'a' IF EXISTS", false, false, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			handled, stmt, err := IsQueryHandled(IdentifierFromString(tt.keyspace), tt.query)
			assert.True(t, (err != nil) == tt.hasError, tt.query)

			idempotent, err := IsQueryIdempotent(tt.query)
			assert.Nil(t, err, tt.query)

			assert.Equal(t, tt.handled, handled, "invalid handled", tt.query)
			assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.query)
			assert.Equal(t, tt.stmt, stmt, "invalid parsed statement", tt.query)
		})
	}
}

func TestParserSystemNowFunction(t *testing.T) {
	var tests = []struct {
		query string
		table string
	}{
		{"SELECT now() FROM system.local", "local"},
		{"SELECT now() FROM system.peers", "peers"},
	}

	start, _, err := uuid.GetTime()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			handled, stmt, err := IsQueryHandled(IdentifierFromString(""), tt.query)
			assert.NoError(t, err, tt.query)
			assert.True(t, handled, tt.query)
			require.NotNil(t, stmt)
			if selectStmt, ok := stmt.(*SelectStatement); ok {
				require.Len(t, selectStmt.Selectors, 1)
				selector := selectStmt.Selectors[0]
				require.NotNil(t, selector)
				if nowSelector, ok := selector.(*NowFuncSelector); ok {
					values, err := nowSelector.Values(nil, nil)
					assert.NoError(t, err)
					assert.Len(t, values, 1)

					var u primitive.UUID
					wasNull, err := datacodec.Timeuuid.Decode(values[0], &u, primitive.ProtocolVersion4)
					assert.False(t, wasNull)
					assert.NoError(t, err)

					v, err := uuid.FromBytes(u[:])
					assert.NoError(t, err)
					assert.Equal(t, uuid.RFC4122, v.Variant())
					assert.Equal(t, uuid.Version(1), v.Version())
					assert.GreaterOrEqual(t, v.Time(), start)

					columns, err := nowSelector.Columns(nil, selectStmt)
					assert.NoError(t, err)
					assert.Len(t, columns, 1)
					assert.Equal(t, "system.now()", columns[0].Name)
					assert.Equal(t, "system", columns[0].Keyspace)
					assert.Equal(t, tt.table, columns[0].Table)
				} else {
					assert.Fail(t, "expected now function selector")
				}
			} else {
				assert.Fail(t, "expected select statement")
			}
		})
	}
}
