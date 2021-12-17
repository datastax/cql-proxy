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
