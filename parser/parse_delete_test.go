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

func TestIsIdempotentDeleteStmt(t *testing.T) {
	var tests = []struct {
		query      string
		idempotent bool
		hasError   bool
		msg        string
	}{
		{"DELETE FROM table", true, false, "simple"},
		{"DELETE FROM table;", true, false, "semicolon at the end"},
		{"DELETE a FROM table", true, false, "w/ operation"},
		{"DELETE a FROM ks.table", true, false, "simple qualified table"},
		{"DELETE a.b FROM table", true, false, "UDT field"},
		{"DELETE a, b, c['key'] FROM table", true, false, "multiple operations"},
		{"DELETE a['key'] FROM ks.table", true, false, "map field"},
		{"DELETE a FROM table WHERE a > 0", true, false, "where clause"},

		// Invalid
		{"DELETE a. FROM table", false, true, "no UDT field"},
		{"DELETE FROM ks.", false, true, "no table after '.'"},
		{"DELETE FROM table WHERE", true, false, "where clause w/ no relation"},
		{"DELETE a[0 table WHERE b > 0", false, true, "collection element with no closing square bracket"},

		// Not idempotent
		{"DELETE a, b, c[1] FROM ks.table", false, false, "multiple with list element"},
		{"DELETE FROM ks.table WHERE a > toTimestamp(now())", false, false, "now() relation"},
		{"DELETE FROM table WHERE a > 0 IF EXISTS", false, false, "LWT"},
		{"DELETE a['key'] FROM table WHERE a > 0 IF EXISTS", false, false, "LWT w/ map field"},
		{"DELETE a['key'] FROM table WHERE a > 0 IF EXISTS;", false, false, "LWT w/ map field and semicolon"},

		// Ambiguous
		{"DELETE a[0] FROM ks.table", false, false, "potentially a list element"},
		{"DELETE a[?] FROM ks.table", false, false, "potentially a list element w/ bind marker"},
		{"DELETE a[func()] FROM ks.table", false, false, "potentially a list element w/ function call"},
	}

	for _, tt := range tests {
		idempotent, err := IsQueryIdempotent(tt.query)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.msg)
	}
}
