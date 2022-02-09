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

func TestIsIdempotentBatchStmt(t *testing.T) {
	var tests = []struct {
		query      string
		idempotent bool
		hasError   bool
		msg        string
	}{
		// Idempotent
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1)
		  APPLY BATCH`,
			true, false, "simple"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1)
		  APPLY BATCH;`,
			true, false, "semicolon at the end of the batch"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1);
			INSERT INTO table (a, b, c) VALUES (2, 'b', 0.2);
		  APPLY BATCH;`,
			true, false, "semicolon at the end of child statements"},
		{`BEGIN BATCH
		 APPLY BATCH`,
			true, false, "empty"},
		{`BEGIN BATCH
			UPDATE ks.table SET b = 0 WHERE a > 100
			DELETE a FROM ks.table
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1)
			INSERT INTO table (a, b, c) VALUES (2, 'b', 0.2)
		  APPLY BATCH`,
			true, false, "multiple statements"},

		// Invalid
		{`BATCH
			SELECT * FROM table
		  APPLY BATCH`,
			false, true, "no starting 'BEGIN'"},
		{`BEGIN BATCH
			SELECT * FROM table
		  APPLY BATCH`,
			false, true, "contains 'SELECT'"},
		{`BEGIN
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) 
		  APPLY BATCH`,
			false, true, "no starting 'BATCH'"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) 
		  BATCH`,
			false, true, "no ending 'APPLY'"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) 
		  APPLY`,
			false, true, "no ending 'BATCH'"},

		// Not idempotent
		{`BEGIN COUNTER BATCH
			INSERT INTO table (a, b) VALUES ('a', 0) 
		  APPLY BATCH`,
			false, false, "batch counter insert"},
		{`BEGIN COUNTER BATCH
			UPDATE table SET a = a + 1
		  APPLY BATCH`,
			false, false, "batch counter update"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) 
			DELETE a, b, c[1] FROM ks.table;
		  APPLY BATCH`,
			false, false, "delete from list in batch"},
		{`BEGIN BATCH
			INSERT INTO table (a, b, c) VALUES (1, 'a', 0.1) 
			INSERT INTO table (a, b, c) VALUES (now(), 'a', 0.1)
		  APPLY BATCH;`,
			false, false, "contains now()"},
		// Found defect
		{"BEGIN BATCH USING TIMESTAMP 1481124356754405\nINSERT INTO cycling.cyclist_expenses \n   (cyclist_name, expense_id, amount, description, paid) \n   VALUES ('Vera ADRIAN', 2, 13.44, 'Lunch', true);\nINSERT INTO cycling.cyclist_expenses \n   (cyclist_name, expense_id, amount, description, paid) \n   VALUES ('Vera ADRIAN', 3, 25.00, 'Dinner', true);\nAPPLY BATCH;",
			true, false, "has semicolons after each statement"},
	}

	for _, tt := range tests {
		idempotent, err := IsQueryIdempotent(tt.query)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.msg)
	}
}
