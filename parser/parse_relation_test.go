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

func TestParseRelation(t *testing.T) {
	var tests = []struct {
		relation   string
		idempotent bool
		hasError   bool
		msg        string
	}{
		{"id > 0", true, false, "simple operator relation"},
		{"token (a, b, c) > (0, 1, 2)", true, false, "'token' relation"},
		{"id LIKE 'abc'", true, false, "'like' relation"},
		{"id IS NOT NULL", true, false, "'is not null' relation"},
		{"id CONTAINS 'abc'", true, false, "'contains' relation"},
		{"id CONTAINS KEY 'abc'", true, false, "'contains key' relation"},
		{"id[0] > 0", true, false, "index collection w/ int relation"},
		{"id['abc'] > 'def'", true, false, "index collection w/string relation"},
		{"id IN ?", true, false, "'IN' w/ position bind marker relation "},
		{"id IN :column", true, false, "'IN' w/ named bind marker relation"},
		{"id IN (0, 1, 2)", true, false, "'IN' w/ terms"},
		{"((((id > 0))))", true, false, "arbitrary number of parens"},
		{"(id1, id2, id3) IN ()", true, false, "list in empty"},
		{"(id1, id2, id3) IN ?", true, false, "list in positional bind marker"},
		{"(id1, id2, id3) IN :named", true, false, "list in named bind marker"},
		{"(id1, id2, id3) IN (?, ?, :named)", true, false, "list in list of bind markers"},
		{"(id1, id2, id3) IN (('a', ?, 0), ('b', :named, 1))", true, false, "list in list of tuples"},
		{"(id1, id2, id3) > ?", true, false, "list in positional bind marker"},
		{"(id1, id2, id3) < :named", true, false, "list in named bind marker"},
		{"(id1, id2, id3) >= (?, ?, :named)", true, false, "list in list of bind markers"},
		{"(id1, id2, id3) <= (('a', ?, 0), ('b', :named, 1))", true, false, "list in list of tuples"},

		// Invalid
		{"id >", false, true, "missing term"},
		{"id == 0", false, true, "invalid operator"},
		{"token a, b, c) > (0, 1, 2)", false, true, "invalid 'token' relation w/ missing identifiers opening paren"},
		{"token (a, b, c > (0, 1, 2)", false, true, "invalid 'token' relation w/ missing identifiers closing paren"},
		{"token (a, b, c) > (0, 1, 2", false, true, "invalid 'token' relation w/ missing tuple closing paren"},
		{"id IS", false, true, "invalid 'is not null' relation"},
		{"id CONTAINS", false, true, "invalid 'contains' relation w/ missing term"},
		{"id CONTAINS KEY", false, true, " invalid 'contains key' relation w/ missing term"},
		{"id[0 > 0", false, true, "invalid index collection w/ int relation w/ missing closing square bracket"},
		{"id[0] >", false, true, "invalid index collection w/ int relation w/ missing term"},
		{"id LIKE", false, true, "invalid 'like' relation w/ missing term"},
		{"id IN", false, true, "invalid 'IN' relation w/ missing bind marker/term"},
		{"id IN 0", false, true, "invalid 'IN' relation w/ unexpect term"},
		{"id IN (", false, true, "invalid 'IN' relation w/ missing closing paren and empty"},
		{"id IN ('a'", false, true, "invalid 'IN' relation w/ missing closing paren"},
		{"(id1, id2)", false, true, "invalid identifiers w/ no operator"},
		{"id1, id2) IN ()", false, true, "invalid identifiers w/ missing opening paren"},
		{"(id1, id2 IN ()", false, true, "invalid identifiers w/ missing closing paren"},
		{"(id1, id2) IN ('a', 1", false, true, "invalid identifiers w/ missing terms closing paren"},
		{"(id1, id2) == ('a', 1)", false, true, "invalid identifiers w/ invalid operator"},

		// Not idempotent
		{"id > now()", false, false, "simple operator relation w/ 'now()'"},
		{"id LIKE now()", false, false, "'like' relation w/ 'now()'"},
		{"id CONTAINS now()", false, false, "'contains' relation w/ 'now()'"},
		{"id CONTAINS KEY now()", false, false, "'contains key' relation w/ 'now()'"},
		{"id1 IN (now(), uuid())", false, false, "'in' relation w/ 'now()'"},
		{"(id1, id2) IN (now(), uuid())", false, false, "list 'IN' relation w/ 'now()'"},
		{"(id1, id2) < (now(), uuid())", false, false, "list operator reation w/ 'now()'"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.relation)
		idempotent, err := parseRelation(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
	}
}
