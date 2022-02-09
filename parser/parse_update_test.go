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

func TestIsIdempotentUpdateStmt(t *testing.T) {
	var tests = []struct {
		query      string
		idempotent bool
		hasError   bool
		msg        string
	}{
		{"UPDATE table SET a = 0", true, false, "simple table"},
		{"UPDATE table SET a = 0;", true, false, "semicolong"},
		{"UPDATE table SET a = 0, b = 0", true, false, "multiple updates"},
		{"UPDATE ks.table SET a = 0", true, false, "simple qualified table"},
		{"UPDATE ks.table USING TIMESTAMP 1234 SET a = 0", true, false, "using timestamp"},
		{"UPDATE ks.table USING TIMESTAMP 1234 AND TTL 5678 SET a = 0", true, false, "using timestamp and ttl"},
		{"UPDATE ks.table USING TTL 1234 SET a = 0", true, false, "using ttl"},
		{"UPDATE ks.table USING TTL 1234 AND TIMESTAMP 5678 SET a = 0", true, false, "using ttl and timestamp"},
		{"UPDATE ks.table SET a = 0 WHERE a > 100", true, false, "where clause"},

		// Invalid
		{"UPDATE table", false, true, "no 'SET'"},
		{"UPDATE table a = 0", false, true, "no 'SET' w/ update operation"},
		{"UPDATE table a = 0 WHERE", false, true, "where clause no relations"},
		{"UPDATE table SET a = 0,", true, false, "multiple updates no operation"},
		{"UPDATE ks. SET a = 0", false, true, "no table"},
		{"UPDATE table USING SET a = 0", false, true, "no timestamp or ttl"},
		{"UPDATE table USING TTL SET a = 0", false, true, "ttl no value"},
		{"UPDATE table USING TTL 1234 AND SET a = 0", false, true, "no ttl/timestamp after 'AND' in using clause"},

		// Not idempotent
		{"UPDATE table SET a = now()", false, false, "simple w/ now()"},
		{"UPDATE table SET a = a + 1", false, false, "add to counter"},
		{"UPDATE table USING TIMESTAMP 1234 SET a = now()", false, false, "using clause w/ now()"},
		{"UPDATE table SET a = 0 WHERE a > toTimestamp(toDate(now()))", false, false, "where clause w/ now()"},
		{"UPDATE table SET a = 0 WHERE a > 0 IF EXISTS", false, false, "where clause w/ LWT"},
		{"UPDATE table SET a = 0 IF EXISTS", false, false, "LWT"},
		{"UPDATE table SET a = 0 IF EXISTS;", false, false, "LWT and semicolon"},
	}

	for _, tt := range tests {
		idempotent, err := IsQueryIdempotent(tt.query)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.msg)
	}
}
