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
		// Not idempotent
		{"id > now()", false, nil, "simple operator relation w/ 'now()'"},
		{"id LIKE now()", false, nil, "'like' relation w/ 'now()'"},
		{"id CONTAINS now()", false, nil, "'contains' relation w/ 'now()'"},
		{"id CONTAINS KEY now()", false, nil, "'contains key' relation w/ 'now()'"},
		{"id1 IN (now(), uuid())", false, nil, "'in' relation w/ 'now()'"},
		{"(id1, id2) IN (now(), uuid())", false, nil, "list 'IN' relation w/ 'now()'"},
		{"(id1, id2) < (now(), uuid())", false, nil, "list operator reation w/ 'now()'"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.relation)
		idempotent, err := parseRelation(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.Equal(t, err, err, tt.msg)
	}
}
