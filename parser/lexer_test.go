package parser

import (
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
		s  string
		tk token
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
		{"''''", tkStringLiteral},
		{"'''", tkInvalid},
		{"$a$", tkStringLiteral},
		{"$$$$", tkStringLiteral},
		{"$$$", tkInvalid},
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
		l.init(tt.s)
		assert.Equal(t, tt.tk, l.next())
	}
}

func TestLexerFloat(t *testing.T) {
	var tests = []string{"0", "1", "-1", "-99999"}
	for _, tt := range tests {
		var l lexer
		l.init(tt)
		assert.Equal(t, tkInteger, l.next())
	}
}
