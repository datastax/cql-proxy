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

func TestParseUpdateOp(t *testing.T) {
	var tests = []struct {
		updateOp   string
		idempotent bool
		hasError   bool
		msg        string
	}{
		{"a = 0", true, false, "simple update operation"},
		{"a['a'] = 0", true, false, "assign to collection"},
		{"a[0] = 0", true, false, "assign to collection (integer index)"},
		{"a.b = 0", true, false, "assign to UDT field"},
		{"a = [1, 2, 3]", true, false, "assign list"},
		{"a = {1, 2, 3}", true, false, "assign set"},
		{"a = {'a': 1, 'b': 2, 'c': 3}", true, false, "assign map"},

		{"a = a + {1, 2}", true, false, "insert into set"},
		{"a += {1, 2}", true, false, "insert (assign) into set"},
		{"a = a - {1, 2}", true, false, "remove from set"},
		{"a -= {1, 2}", true, false, "remove (assign) from set"},

		{"a = a + {'a': 1, 'b': 2}", true, false, "insert into map"},
		{"a += {'a': 1, 'b': 2}", true, false, "insert (assign) into map"},
		{"a = a - {'a': 1, 'b': 2}", true, false, "remove from map"},
		{"a -= {'a': 1, 'b': 2}", true, false, "remove (assign) from map"},

		{"a = a + (1, 'a')", true, false, "insert into tuple"},
		{"a += (1, 'a')", true, false, "insert (assign) into tuple"},
		{"a = a - (1, 'a')", true, false, "remove from tuple"},
		{"a -= (1, 'a')", true, false, "remove (assign) from tuple"},

		// Invalid
		{"0 = a", false, true, "start w/ term"},
		{"a[0 = a", false, true, "no closing square bracket"},
		{"a0] = a", false, true, "no opening square bracket"},
		{"a. = a", false, true, "no identifier after '.' for UDT field"},

		{"a = a +", false, true, "add with no term"},
		{"a = a -", false, true, "subtract with no term"},
		{"a = 1 +", false, true, "add with no identifier"},
		{"a +=", false, true, "add assign with no term"},
		{"a -=", false, true, "subtract assign with no term"},

		// Not idempotent
		{"a = now()", false, false, "simple update operation w/ now()"},
		{"a[0] = now()", false, false, "assign to collection (integer index) w/ now()"},
		{"a[now()] = 0", false, false, "assign to collection w/ now() index"},
		{"a.b = now()", false, false, "assign to UDT field with now()"},

		{"a = a + 1", false, false, "add to counter"},
		{"a += 1", false, false, "add assign to counter"},
		{"a = 1 + a", false, false, "add to counter swap term"},
		{"a = a - 1", false, false, "subtract from counter"},
		{"a -= 1", false, false, "subtract assign from counter"},

		{"a = a + [1, 2]", false, false, "append to list"},
		{"a += [1, 2]", false, false, "append assign to list"},
		{"a = [1, 2] + a", false, false, "prepend to list"},

		{"a = a - [1, 2]", false, false, "remove from list"},
		{"a -= [1, 2]", false, false, "remove assign to list"},

		{"a = a + (int)1", false, false, "add/append w/ cast"},
		{"a += (int)1", false, false, "add/append assign w/ cast"},
		{"a = (int)1 + a", false, false, "add/append (swap term) w/ cast"},
		{"a = a - (int)1", false, false, "subtract/remove w/ cast"},
		{"a -= (int)1", false, false, "subtract/remove assign w/ cast"},

		// Ambiguous
		{"a = a + ?", false, false, "add/append w/ bind marker"},
		{"a += ?", false, false, "add/append assign w/ bind marker"},
		{"a = ? + a", false, false, "add/append (swap term) w/ bind marker"},
		{"a = a - ?", false, false, "subtract/remove w/ bind marker"},
		{"a -= ?", false, false, "subtract/remove assign w/ bind marker"},

		{"a = a + :name", false, false, "add/append w/ named bind marker"},
		{"a += :name", false, false, "add/append assign w/ named bind marker"},
		{"a = :name + a", false, false, "add/append (swap term) w/ named bind marker"},
		{"a = a - :name", false, false, "subtract/remove w/ named bind marker"},
		{"a -= :name", false, false, "subtract/remove assign w/ named bind marker"},

		{"a = a + func()", false, false, "add/append w/ function"},
		{"a += func()", false, false, "add/append assign w/ function"},
		{"a = func() + a", false, false, "add/append (swap term) w/ function"},
		{"a = a - func()", false, false, "subtract/remove w/ function"},
		{"a -= func()", false, false, "subtract/remove assign w/ function"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.updateOp)
		idempotent, err := parseUpdateOp(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
	}
}
