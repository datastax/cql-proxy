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
