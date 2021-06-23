package parser

import (
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

type StmtType int

const (
	StmtUnknown StmtType = iota
	StmtSelect
	StmtUse
)

type StmtExprType int

const (
	StmtExprID StmtExprType = iota
	StmtExprAlias
	StmtExprStar
	StmtExprCount
)

type StmtExpr struct {
	typ   StmtExprType
	id    string
	alias string
}

type Stmt struct {
	typ         StmtType
	isTableOnly bool
	table       string
}

func Parse(query string) (StmtType, string) {
	lexer := NewCqlLexer(antlr.NewInputStream(query))
	switch nextToken(lexer).GetTokenType() {
	case CqlLexerK_SELECT:
		return parseSelect(lexer)
	case CqlLexerK_USE:
		return parseUse(lexer)
	}
	return StmtUnknown, ""
}

func nextToken(lexer *CqlLexer) antlr.Token {
	var token antlr.Token
	for token = lexer.NextToken(); token.GetTokenType() == CqlLexerSPACE; {
		token = lexer.NextToken()
	}
	return token
}

func parseExpr(lexer antlr.Lexer, token antlr.Token) antlr.Token {
}

func parseIdentifier(lexer *CqlLexer, token antlr.Token) string {
	var id string
	if token.GetTokenType() == CqlLexerOBJECT_NAME {
		id = strings.ToLower(token.GetText())
	} else if token.GetTokenType() == CqlLexerDQUOTE {
		token = nextToken(lexer)
		if token.GetTokenType() == CqlLexerOBJECT_NAME {
			tmp := token.GetText()
			token = nextToken(lexer)
			if token.GetTokenType() == CqlLexerDQUOTE {
				id = tmp
			}
		}
	}
	return id
}

func parseSelect(lexer *CqlLexer) (StmtType, string) {
	token := nextToken(lexer)
	for token.GetTokenType() != CqlLexerK_FROM && token.GetTokenType() != antlr.TokenEOF {
		token = nextToken(lexer)
	}
	if token.GetTokenType() != CqlLexerK_FROM {
		return StmtUnknown, ""
	}
	tableOrKeyspace := parseIdentifier(lexer, nextToken(lexer))
	if tableOrKeyspace == "" {
		return StmtUnknown, ""
	}
	token = nextToken(lexer)
	if token.GetTokenType() == CqlLexerDOT {
		token = nextToken(lexer)
		table := parseIdentifier(lexer, token)
		if table == "" {
			return StmtUnknown, ""
		}
		return StmtSelect, tableOrKeyspace + "." + table
	} else {
		return StmtSelect, tableOrKeyspace
	}
}

func parseUse(lexer *CqlLexer) (StmtType, string) {
	keyspace := parseIdentifier(lexer, nextToken(lexer))
	if keyspace == "" {
		return StmtUnknown, ""
	}
	return StmtUse, keyspace
}
