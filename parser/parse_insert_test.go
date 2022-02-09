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

	"github.com/stretchr/testify/assert"
)

func TestIsIdempotentInsertStmt(t *testing.T) {
	var tests = []struct {
		query      string
		idempotent bool
		hasError   bool
		msg        string
	}{
		{"INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1)", true, false, "simple"},
		{"INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1);", true, false, "semicolon"},
		{"INSERT INTO ks.table (a, b, c) VALUES (1, 'a', 0.1)", true, false, "simple qualified table name"},
		{"INSERT INTO table () VALUES ()", true, false, "no identifier of values"},
		{"INSERT INTO table JSON '{}'", true, false, "JSON"},

		// Invalid
		{"INSERT table (a, b, c) VALUES (1, 'a', 0.1)", false, true, "missing 'INTO'"},
		{"INSERT INTO (a, b, c) VALUES (1, 'a', 0.1)", false, true, "missing table name"},
		{"INSERT INTO table a, b, c) VALUES (1, 'a', 0.1)", false, true, "missing opening paren. on identifiers"},
		{"INSERT INTO table (a, b, c VALUES (1, 'a', 0.1)", false, true, "missing closing paren on identifiers"},
		{"INSERT INTO table (a, b, c) (1, 'a', 0.1)", false, true, "missing 'VALUES'"},
		{"INSERT INTO table (0, b, c) VALUES (1, 'a', 0.1)", false, true, "unexpected term in identifiers"},
		{"INSERT INTO table (a, b, c) VALUES (invalid, 'a', 0.1)", false, true, "invalid value"},
		{"INSERT INTO table (a, b, c) VALUES 1, 'a', 0.1)", false, true, "missing opening paren. on values"},
		{"INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1", false, true, "missing closing paren. on values"},

		// Not idempotent
		{"INSERT INTO table (a, b, c) VALUES (now(), 'a', 0.1)", false, false, "simple w/ 'now()'"},
		{"INSERT INTO table (a, b, c) VALUES (0, uuid(), 0.1)", false, false, "simple w/ 'uuid()'"},
		{"INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) IF NOT EXISTS", false, false, "simple w/ LWT"},
		{"INSERT INTO table () VALUES () IF NOT EXIST", false, false, "no identifier of values w/ LWT"},
		{"INSERT INTO table JSON '{}' IF NOT EXIST", false, false, "'JSON' w/ LWT"},
		{"INSERT INTO table JSON '{}' IF NOT EXIST;", false, false, "'JSON' w/ LWT and semicolon"},
	}

	for _, tt := range tests {
		idempotent, err := IsQueryIdempotent(tt.query)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.msg)
	}
}
