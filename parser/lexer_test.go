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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexerNext(t *testing.T) {
	var l lexer
	l.init("SELECT * FROM system.local")

	assert.Equal(t, tkSelect, l.next())
	assert.Equal(t, tkStar, l.next())
	assert.Equal(t, tkFrom, l.next())
	assert.Equal(t, tkIdentifier, l.next())
	assert.Equal(t, tkDot, l.next())
	assert.Equal(t, tkIdentifier, l.next())
	assert.Equal(t, tkEOF, l.next())
}

func TestLexerLiterals(t *testing.T) {
	var tests = []struct {
		literal string
		tk      token
	}{
		{"0", tkInteger},
		{"1", tkInteger},
		{"-1", tkInteger},
		{"1.", tkFloat},
		{"0.0", tkFloat},
		{"-0.0", tkFloat},
		{"-1.e9", tkFloat},
		{"-1.e+0", tkFloat},
		{"tRue", tkBool},
		{"False", tkBool},
		{"'a'", tkStringLiteral},
		{"'abc'", tkStringLiteral},
		{"''''", tkStringLiteral},
		{"$a$", tkStringLiteral},
		{"$abc$", tkStringLiteral},
		{"$$$$", tkStringLiteral},
		{"0x", tkHexNumber},
		{"0x0", tkHexNumber},
		{"0xabcdef", tkHexNumber},
		{"123e4567-e89b-12d3-a456-426614174000", tkUuid},
		{"nan", tkNan},
		{"-NaN", tkNan},
		{"-infinity", tkInfinity},
		{"-Infinity", tkInfinity},
		{"1Y", tkDuration},
		{"1µs", tkDuration},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.literal)
		assert.Equal(t, tt.tk, l.next(), fmt.Sprintf("failed on literal: %s", tt.literal))
	}
}

func TestLexerIdentifiers(t *testing.T) {
	var tests = []struct {
		literal  string
		tk       token
		expected string
	}{
		{`system`, tkIdentifier, "system"},
		{`sys"tem`, tkIdentifier, "sys"},
		{`System`, tkIdentifier, "system"},
		{`"system"`, tkIdentifier, "system"},
		{`"system"`, tkIdentifier, "system"},
		{`"System"`, tkIdentifier, "System"},
		// below test verify correct escaping double quote character as per CQL definition:
		//    identifier ::= unquoted_identifier | quoted_identifier
		//    unquoted_identifier ::= re('[a-zA-Z][link:[a-zA-Z0-9]]*')
		//    quoted_identifier ::= '"' (any character where " can appear if doubled)+ '"'
		{`""""`, tkIdentifier, "\""},       // outermost quotes indicate quoted string, inner two double quotes shall be treated as single quote
		{`""""""`, tkIdentifier, "\"\""},   // same as above, but 4 inner quotes result in 2 quotes
		{`"A"""""`, tkIdentifier, "A\"\""}, // outermost quotes indicate quoted string, 4 quotes after A result in 2 quotes
		{`"""A"""`, tkIdentifier, "\"A\""}, // outermost quotes indicate quoted string, 2 quotes before and after A result in single quotes
		{`"""""A"`, tkIdentifier, "\"\"A"}, // analogical to previous tests
		{`";`, tkInvalid, ""},
		{`"""`, tkIdentifier, ""},
	}

	for _, tt := range tests {
		var l lexer
		l.init(tt.literal)
		n := l.next()
		assert.Equal(t, tt.tk, n, fmt.Sprintf("failed on literal: %s", tt.literal))
		if n == tkIdentifier {
			id := l.identifier()
			if id.ID() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, l.id)
			}
		}
	}
}
