package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexer_Next(t *testing.T) {
	var l Lexer
	l.Init("SELECT * FROM system.local")

	assert.Equal(t, TkSelect, l.Next())
	assert.Equal(t, TkStar, l.Next())
	assert.Equal(t, TkFrom, l.Next())
	assert.Equal(t, TkSystemIdentifier, l.Next())
	assert.Equal(t, TkDot, l.Next())
	assert.Equal(t, TkLocalIdentifier, l.Next())
	assert.Equal(t, TkEOF, l.Next())
}
