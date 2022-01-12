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

func TestParseTerm(t *testing.T) {
	var tests = []struct {
		term       string
		idempotent bool
		typ        termType
		hasError   bool
		msg        string
	}{
		{"system.someOtherFunc()", true, termFunctionCall, false, "qualified idempotent function"},
		{"[1, 2, 3]", true, termListLiteral, false, "list literal"},
		{"123", true, termIntegerLiteral, false, "integer literal"},
		{"{1, 2, 3}", true, termSetMapUdtLiteral, false, "set literal"},
		{"{ a: 1, a.b: 2, c: 3}", true, termSetMapUdtLiteral, false, "UDT literal"},
		{"{ 'a': 1, 'b': 2, 'c': 3}", true, termSetMapUdtLiteral, false, "map literal"},
		{"(1, 'a', [])", true, termTupleLiteral, false, "tuple literal"},
		{":abc", true, termBindMarker, false, "named bind marker"},
		{"?", true, termBindMarker, false, "positional bind marker"},
		{"(map<int, text>)1", true, termCast, false, "cast to a complex type"},
		{"func(a, b, c)", true, termFunctionCall, false, "function with identifier args"},

		// Invalid
		{"system.someOtherFunc", false, termFunctionCall, true, "invalid qualified function"},
		{"func", false, termFunctionCall, true, "invalid function"},
		{"[1, 2, 3", false, termListLiteral, true, "invalid list literal"},
		{"{1, 2, 3", false, termSetMapUdtLiteral, true, "invalid set literal"},
		{"{ a: 1, a.b: 2, c: 3", false, termSetMapUdtLiteral, true, "invalid UDT literal"},
		{"{ 'a': 1, 'b': 2, 'c': 3", false, termSetMapUdtLiteral, true, "invalid map literal"},
		{"+123", false, termInvalid, true, "invalid term"},

		// Not idempotent
		{"system.now()", false, termFunctionCall, false, "qualified 'now()' function"},
		{"system.uuid()", false, termFunctionCall, false, "qualified 'uuid()' function "},
		{"(uuid)now()", false, termCast, false, "cast of the 'now()' function"},
		{"now(a, b, c, '1', 0)", false, termFunctionCall, false, "'now()' function w/ args"},
		{"[now(), 2, 3]", false, termListLiteral, false, "list literal with 'now()' function"},
		{"{now():'a'}", false, termSetMapUdtLiteral, false, "map literal with 'now()' function"},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.term)
		idempotent, typ, err := parseTerm(&l, l.next())
		assert.Equal(t, tt.idempotent, idempotent, tt.msg)
		assert.Equal(t, tt.typ, typ, tt.msg)
		assert.True(t, (err != nil) == tt.hasError, tt.msg)
	}
}
