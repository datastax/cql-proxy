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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTerm(t *testing.T) {
	var tests = []struct {
		term       string
		idempotent bool
		typ        termType
		err        error
		msg        string
	}{
		{"system.now()", false, termFunctionCall, nil, "qualified 'now()' function"},
		{"system.uuid()", false, termFunctionCall, nil, "qualified 'uuid()' function "},
		{"system.someOtherFunc()", true, termFunctionCall, nil, "qualified idempotent function"},
		{"[1, 2, 3]", true, termListLiteral, nil, "list literal"},
		{"[now(), 2, 3]", false, termInvalid, nil, "list literal with 'now()' function"},
		{"123", true, termIntegerLiteral, nil, "integer literal"},
		{"{1, 2, 3}", true, termSetMapUdtLiteral, nil, "set literal"},
		{"{ a: 1, a.b: 2, c: 3}", true, termSetMapUdtLiteral, nil, "UDT literal"},
		{"{ 'a': 1, 'b': 2, 'c': 3}", true, termSetMapUdtLiteral, nil, "map literal"},
		{"(1, 'a', [])", true, termTupleLiteral, nil, "tuple literal"},
		{":abc", true, termBindMarker, nil, "named bind marker"},
		{"?", true, termBindMarker, nil, "positional bind marker"},
		{"(map<int, text>)1", true, termCast, nil, "cast to a complex type"},
		{"(uuid)now()", false, termInvalid, nil, "cast of the 'now()' function"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.term)
		idempotent, typ, err := parseTerm(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.Equal(t, tt.typ, typ, tt.msg)
		assert.Equal(t, err, err, tt.msg)
	}
}

func TestParseRelation(t *testing.T) {
	var tests = []struct {
		relation   string
		idempotent bool
		err        error
		msg        string
	}{
		{"id > 0", true, nil, "simple operator relation"},
		{"token (a, b, c) > (0, 1, 2)", true, nil, "'token' relation"},
		{"id LIKE 'abc'", true, nil, "'like' relation"},
		{"id IS NOT NULL", true, nil, "'is not null' relation"},
		{"id CONTAINS 'abc'", true, nil, "'contains' relation"},
		{"id CONTAINS KEY 'abc'", true, nil, "'contains key' relation"},
		{"id[0] > 0", true, nil, "index collection w/ int relation"},
		{"id['abc'] > 'def'", true, nil, "index collection w/string relation"},
		{"id IN ?", true, nil, "'IN' w/ position bind marker relation "},
		{"id IN :column", true, nil, "'IN' w/ named bind marker relation"},
		{"((((id > 0))))", true, nil, "arbitrary number of parens"},
		{"(id1, id2, id3) IN ()", true, nil, "list in empty"},
		{"(id1, id2, id3) IN ?", true, nil, "list in positional bind marker"},
		{"(id1, id2, id3) IN :named", true, nil, "list in named bind marker"},
		{"(id1, id2, id3) IN (?, ?, :named)", true, nil, "list in list of bind markers"},
		{"(id1, id2, id3) IN (('a', ?, 0), ('b', :named, 1))", true, nil, "list in list of tuples"},
		{"(id1, id2, id3) > ?", true, nil, "list in positional bind marker"},
		{"(id1, id2, id3) < :named", true, nil, "list in named bind marker"},
		{"(id1, id2, id3) >= (?, ?, :named)", true, nil, "list in list of bind markers"},
		{"(id1, id2, id3) <= (('a', ?, 0), ('b', :named, 1))", true, nil, "list in list of tuples"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.relation)
		idempotent, err := parseRelation(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.Equal(t, err, err, tt.msg)
	}

}

func TestParser(t *testing.T) {
	var tests = []struct {
		keyspace   string
		query      string
		handled    bool
		idempotent bool
		stmt       Statement
		err        error
	}{
		{"", "SELECT key, rpc_address AS address, count(*) FROM system.local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []Selector{
				&IDSelector{Name: "key"},
				&AliasSelector{Alias: "address", Selector: &IDSelector{Name: "rpc_address"}},
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT count(*) FROM system.peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT count(*) FROM system.peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT func(key) FROM system.local", true, true, nil,
			errors.New("unsupported select clause for system table")},
		{"", "USE system", true, false, &UseStatement{
			Keyspace: "system",
		}, nil},
		// Reads from tables named similarly to system tables (not handled)
		{"", "SELECT count(*) FROM local", false, true, nil, nil},
		{"", "SELECT count(*) FROM peers", false, true, nil, nil},
		{"", "SELECT count(*) FROM peers_v2", false, true, nil, nil},

		// Mutations to system tables (not handled)
		{"", "INSERT INTO system.local (key, rpc_address) VALUES ('local1', '127.0.0.1')", false, true, nil, nil},
		{"", "UPDATE system.local SET rpc_address = '127.0.0.1' WHERE key = 'local'", false, true, nil, nil},
		{"", "DELETE rpc_address FROM system.local WHERE key = 'local'", false, true, nil, nil},

		// Mutations that use the functions whose values change e.g. uuid(), now() (not idempotent)
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', uuid())", false, false, nil, nil},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', now())", false, false, nil, nil},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', nested(now()))", false, false, nil, nil},
		{"", "INSERT INTO ks.table (key, value) VALUES ('k1', nested(uuid()))", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = uuid() WHERE k = 1", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = now() WHERE k = 1", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = nested(uuid()) WHERE k = 1", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = nested(now()) WHERE k = 1", false, false, nil, nil},

		// Updates that prepend/append to a list (not idempotent)
		{"", "UPDATE ks.table SET v = v + [1] WHERE k = 1", false, false, nil, nil}, // Append
		{"", "UPDATE ks.table SET v = [1] + v WHERE k = 1", false, false, nil, nil}, // Prepend
		{"", "UPDATE ks.table SET v += [1] WHERE k = 1", false, false, nil, nil},    // Append assign
		{"", "UPDATE ks.table SET v -= [1] WHERE k = 1", false, false, nil, nil},    // Remove assign

		// Updates to counter values (not idempotent)
		{"", "UPDATE ks.table SET v = v + 1 WHERE k = 1", false, false, nil, nil}, // Left add
		{"", "UPDATE ks.table SET v = 1 + v WHERE k = 1", false, false, nil, nil}, // Right add
		{"", "UPDATE ks.table SET v += 1 WHERE k = 1", false, false, nil, nil},    // Add assign
		{"", "UPDATE ks.table SET v = v - 1 WHERE k = 1", false, false, nil, nil}, // Left subtract
		{"", "UPDATE ks.table SET v -= 1 WHERE k = 1", false, false, nil, nil},    // Subtract assign

		// Update set/map (idempotent)
		{"", "UPDATE ks.table SET v = v + { 1 } WHERE k = 1", false, true, nil, nil},        // Add to set (right)
		{"", "UPDATE ks.table SET v = { 1 } + v WHERE k = 1", false, true, nil, nil},        // Add to set (left)
		{"", "UPDATE ks.table SET v = v + { 'a': 1 } WHERE k = 1", false, true, nil, nil},   // Add to map (right)
		{"", "UPDATE ks.table SET v =  { 'a': 1 } +  v WHERE k = 1", false, true, nil, nil}, // Add to map (left)

		// Deletes to elements of a collection (not idempotent)
		{"", "DELETE v[0] FROM ks.table WHERE k = 1", false, false, nil, nil},

		// Deletes to elements of a collection (idempotent)
		{"", "DELETE v['a'] FROM ks.table WHERE k = 1", false, true, nil, nil},                                  // String
		{"", "DELETE v[0.0] FROM ks.table WHERE k = 1", false, true, nil, nil},                                  // Float
		{"", "DELETE v[0x1] FROM ks.table WHERE k = 1", false, true, nil, nil},                                  // Hex (which is different from int)
		{"", "DELETE v[true] FROM ks.table WHERE k = 1", false, true, nil, nil},                                 // Boolean true
		{"", "DELETE v[false] FROM ks.table WHERE k = 1", false, true, nil, nil},                                // Boolean false
		{"", "DELETE v[(1,2,3)] FROM ks.table WHERE k = 1", false, true, nil, nil},                              // Tuple
		{"", "DELETE v[{field1: 1, field2: 'a'}] FROM ks.table WHERE k = 1", false, true, nil, nil},             // UDT
		{"", "DELETE v[[1,2,3]] FROM ks.table WHERE k = 1", false, true, nil, nil},                              // List
		{"", "DELETE v[{1,2,3}] FROM ks.table WHERE k = 1", false, true, nil, nil},                              // Set
		{"", "DELETE v[{'a': 1}] FROM ks.table WHERE k = 1", false, true, nil, nil},                             // Map
		{"", "DELETE v[Nan] FROM ks.table WHERE k = 1", false, true, nil, nil},                                  // Nan (float)
		{"", "DELETE v[Infinity] FROM ks.table WHERE k = 1", false, true, nil, nil},                             // Infinity (float)
		{"", "DELETE v[null] FROM ks.table WHERE k = 1", false, true, nil, nil},                                 // Null
		{"", "DELETE v[2021Y12M03D] FROM ks.table WHERE k = 1", false, true, nil, nil},                          // Duration
		{"", "DELETE v[123e4567-e89b-12d3-a456-426614174000] FROM ks.table WHERE k = 1", false, true, nil, nil}, // UUID

		// Lightweight transactions (LWTs are not idempotent)
		{"", "INSERT INTO ks.table (k, v) VALUES ('a', 1) IF NOT EXISTS", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF v > 2", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF EXISTS", false, false, nil, nil},
		{"", "UPDATE ks.table SET v = 1 WHERE k = 'a' IF NOT EXISTS", false, false, nil, nil},
		{"", "DELETE FROM ks.table WHERE k = 'a' IF EXISTS", false, false, nil, nil},
		{"", "DELETE a.b, c.d FROM ks.table WHERE k = 'a' IF EXISTS", false, false, nil, nil},
	}

	for _, tt := range tests {
		handled, stmt, err := IsQueryHandled(tt.keyspace, tt.query)
		assert.Equal(t, tt.err, err, tt.query)

		idempotent, err := IsQueryIdempotent(tt.query)
		assert.Nil(t, err, tt.query)

		assert.Equal(t, tt.handled, handled, "invalid handled", tt.query)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.query)
		assert.Equal(t, tt.stmt, stmt, "invalid parsed statement", tt.query)
	}
}
