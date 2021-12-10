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

//go:generate ragel -Z -G2 lexer.rl -o lexer.go

package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	parser "github.com/datastax/cql-proxy/parser/antlr"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	CountValueName = "count(*)"
)

type parseState uint32

const (
	inSelectStatement parseState = 1 << iota
	inSelectCause
	inInsertStatement
	inInsertTerms
	inFunctionName
	inUpdateStatement
	inUpdateOperations
	inDeleteStatement
)

var systemTables = []string{"local", "peers", "peers_v2", "schema_keyspaces", "schema_columnfamilies", "schema_columns", "schema_usertypes"}

type AliasSelector struct {
	Selector interface{}
	Alias    string
}

type IDSelector struct {
	Name string
}

type StarSelector struct{}

type CountStarSelector struct {
	Name string
}

type ErrorSelectStatement struct {
	Err error
}

type SelectStatement struct {
	Table     string
	Selectors []interface{}
}

type UseStatement struct {
	Keyspace string
}

type ValueLookupFunc func(name string) (value message.Column, err error)

func Parse(keyspace string, query string) (handled bool, idempotent bool, stmt interface{}) {
	is := antlr.NewInputStream(query)
	lexer := parser.NewSimplifiedCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	cqlParser := parser.NewSimplifiedCqlParser(stream)
	listener := &queryListener{keyspace: keyspace}
	antlr.ParseTreeWalkerDefault.Walk(listener, cqlParser.CqlStatement())
	return listener.handled, listener.idempotent, listener.stmt
}

func FilterValues(stmt *SelectStatement, columns []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	if _, ok := stmt.Selectors[0].(*StarSelector); ok {
		for _, column := range columns {
			var val message.Column
			val, err = valueFunc(column.Name)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, val)
		}
	} else {
		for _, selector := range stmt.Selectors {
			var val message.Column
			val, err = valueFromSelector(selector, valueFunc)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, val)
		}
	}
	return filtered, nil
}

func valueFromSelector(selector interface{}, valueFunc ValueLookupFunc) (val message.Column, err error) {
	switch s := selector.(type) {
	case *CountStarSelector:
		return valueFunc(CountValueName)
	case *IDSelector:
		return valueFunc(s.Name)
	case *AliasSelector:
		return valueFromSelector(s.Selector, valueFunc)
	default:
		return nil, errors.New("unhandled selector type")
	}
}

func FilterColumns(stmt *SelectStatement, columns []*message.ColumnMetadata) (filtered []*message.ColumnMetadata, err error) {
	if _, ok := stmt.Selectors[0].(*StarSelector); ok {
		filtered = columns
	} else {
		for _, selector := range stmt.Selectors {
			var column *message.ColumnMetadata
			column, err = columnFromSelector(selector, columns, stmt.Table)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, column)
		}
	}
	return filtered, nil
}

func isCountSelector(selector interface{}) bool {
	_, ok := selector.(*CountStarSelector)
	return ok
}

func IsCountStarQuery(stmt *SelectStatement) bool {
	if len(stmt.Selectors) == 1 {
		if isCountSelector(stmt.Selectors[0]) {
			return true
		} else if alias, ok := stmt.Selectors[0].(*AliasSelector); ok {
			return isCountSelector(alias.Selector)
		}
	}
	return false
}

func columnFromSelector(selector interface{}, columns []*message.ColumnMetadata, table string) (column *message.ColumnMetadata, err error) {
	switch s := selector.(type) {
	case *CountStarSelector:
		return &message.ColumnMetadata{
			Keyspace: "system",
			Table:    table,
			Name:     s.Name,
			Type:     datatype.Int,
		}, nil
	case *IDSelector:
		if column = FindColumnMetadata(columns, s.Name); column != nil {
			return column, nil
		} else {
			return nil, fmt.Errorf("invalid column %s", s.Name)
		}
	case *AliasSelector:
		column, err = columnFromSelector(s.Selector, columns, table)
		if err != nil {
			return nil, err
		}
		alias := *column // Make a copy so we can modify the name
		alias.Name = s.Alias
		return &alias, nil
	default:
		return nil, errors.New("unhandled selector type")
	}
}

type queryListener struct {
	*parser.BaseSimplifiedCqlListener
	keyspace   string
	handled    bool
	idempotent bool
	stmt       interface{}
	parseState parseState
}

func isSystemTable(name string) bool {
	for _, table := range systemTables {
		if table == name {
			return true
		}
	}
	return false
}

func (l *queryListener) EnterSelectStatement(ctx *parser.SelectStatementContext) {
	l.idempotent = true

	tableNameCxt := ctx.TableName().(*parser.TableNameContext).QualifiedIdentifier().(*parser.QualifiedIdentifierContext)
	var keyspace string
	if tableNameCxt.KeyspaceName() != nil {
		keyspace = extractIdentifier(tableNameCxt.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))
	}

	table := extractIdentifier(tableNameCxt.Identifier().(*parser.IdentifierContext))

	if (keyspace == "system" || l.keyspace == "system") && isSystemTable(table) {
		l.handled = true

		selectStmt := &SelectStatement{
			Table:     table,
			Selectors: make([]interface{}, 0),
		}

		selectClauseCtx := ctx.SelectClause().(*parser.SelectClauseContext)

		if selectClauseCtx.Selectors() != nil {
			selectorsCtx := selectClauseCtx.Selectors().(*parser.SelectorsContext)
			for _, selector := range selectorsCtx.AllSelector() {
				selectorCtx := selector.(*parser.SelectorContext)
				unaliasedSelector, err := extractUnaliasedSelector(selectorCtx.UnaliasedSelector().(*parser.UnaliasedSelectorContext))
				if err != nil {
					l.stmt = &ErrorSelectStatement{err}
					return // invalid selector
				}
				if selectorCtx.K_AS() != nil { // alias
					selectStmt.Selectors = append(selectStmt.Selectors, &AliasSelector{
						Selector: unaliasedSelector,
						Alias:    extractIdentifier(selectorCtx.Identifier().(*parser.IdentifierContext)),
					})
				} else {
					selectStmt.Selectors = append(selectStmt.Selectors, unaliasedSelector)
				}
			}
		} else { // 'SELECT * ...'
			selectStmt.Selectors = append(selectStmt.Selectors, &StarSelector{})
		}

		l.stmt = selectStmt
	}
}

func (l *queryListener) EnterUseStatement(ctx *parser.UseStatementContext) {
	l.handled = true
	l.stmt = &UseStatement{Keyspace: extractIdentifier(ctx.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))}
}

//func (l *queryListener) EnterInsertStatement(ctx *parser.InsertStatementContext) {
//	l.idempotent = true
//	l.parseState |= inInsertStatement
//
//	if ctx.K_IF() != nil { // Lightweight transactions are *NOT* idempotent
//		l.idempotent = false
//	}
//}
//
//func (l *queryListener) ExitInsertStatement(_ *parser.InsertStatementContext) {
//	l.parseState &= ^inInsertStatement
//}
//
//func (l *queryListener) EnterUpdateStatement(ctx *parser.UpdateStatementContext) {
//	l.idempotent = true
//	l.parseState |= inUpdateStatement
//
//	if ctx.K_IF() != nil { // Lightweight transactions are *NOT* idempotent
//		l.idempotent = false
//	}
//}
//
//func (l *queryListener) ExitUpdateStatement(_ *parser.UpdateStatementContext) {
//	l.parseState &= ^inUpdateStatement
//}
//
//func (l *queryListener) EnterDeleteStatement(ctx *parser.DeleteStatementContext) {
//	l.idempotent = true
//	l.parseState |= inDeleteStatement
//
//	if ctx.K_IF() != nil { // Lightweight transactions are *NOT* idempotent
//		l.idempotent = false
//	}
//}
//
//func (l *queryListener) EnterUpdateOperations(_ *parser.UpdateOperationsContext) {
//	l.parseState |= inUpdateOperations
//}
//
//func (l *queryListener) ExitUpdateOperations(_ *parser.UpdateOperationsContext) {
//	l.parseState &= ^inUpdateOperations
//}
//
//func (l *queryListener) EnterUpdateOperatorAddLeft(ctx *parser.UpdateOperatorAddLeftContext) {
//	l.idempotent = isIdempotentUpdateTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterUpdateOperatorAddRight(ctx *parser.UpdateOperatorAddRightContext) {
//	l.idempotent = isIdempotentUpdateTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterUpdateOperatorSubtract(ctx *parser.UpdateOperatorSubtractContext) {
//	l.idempotent = isIdempotentUpdateTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterUpdateOperatorAddAssign(ctx *parser.UpdateOperatorAddAssignContext) {
//	l.idempotent = isIdempotentUpdateTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterUpdateOperatorSubtractAssign(ctx *parser.UpdateOperatorSubtractAssignContext) {
//	l.idempotent = isIdempotentUpdateTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterDeleteOperationElement(ctx *parser.DeleteOperationElementContext) {
//	// It's not possible to determine if this is a list element being deleted, so it's *NOT* idempotent.
//	l.idempotent = isIdempotentDeleteElementTerm(ctx.Term().(*parser.TermContext))
//}
//
//func (l *queryListener) EnterInsertTerms(_ *parser.InsertTermsContext) {
//	l.parseState |= inInsertTerms
//}
//
//func (l *queryListener) ExitInsertTerms(_ *parser.InsertTermsContext) {
//	l.parseState &= ^inInsertTerms
//}
//
//func (l *queryListener) EnterFunctionName(_ *parser.FunctionNameContext) {
//	l.parseState |= inFunctionName
//}
//
//func (l *queryListener) ExitFunctionName(_ *parser.FunctionNameContext) {
//	l.parseState &= ^inFunctionName
//}
//
//func (l *queryListener) EnterIdentifier(ctx *parser.IdentifierContext) {
//	// Queries that use the functions `uuid()` or `now()` in writes are *NOT* idempotent
//	if (l.parseState&(inInsertStatement|inInsertTerms|inFunctionName) > 0) ||
//		(l.parseState&(inUpdateStatement|inUpdateOperations|inFunctionName) > 0) {
//		funcName := strings.ToLower(extractIdentifier(ctx))
//		if funcName == "uuid" || funcName == "now" {
//			l.idempotent = false
//		}
//	}
//}
//
//func isIdempotentUpdateTerm(ctx *parser.TermContext) bool {
//	// Update terms can be one of the following:
//	// * Literal (maybe idempotent)
//	// * Bind marker (ambiguous)
//	// * Function call (ambiguous)
//	// * Type cast (probably not idempotent)
//	return ctx.Literal() != nil && isIdempotentUpdateLiteral(ctx.Literal().(*parser.LiteralContext))
//}
//
//func isIdempotentUpdateLiteral(ctx *parser.LiteralContext) bool {
//	// Update literals can be one of the following:
//	// * Primitive (probably not idempotent)
//	// * Collection (maybe idempotent)
//	// * Tuple (idempotent)
//	// * UDT (idempotent)
//	// * `null` (likely not valid)
//	if ctx.UdtLiteral() != nil || ctx.TupleLiteral() != nil {
//		return true
//	} else if ctx.CollectionLiteral() != nil {
//		return isIdempotentUpdateCollectionLiteral(ctx.CollectionLiteral().(*parser.CollectionLiteralContext))
//	}
//	return false
//}
//
//func isIdempotentUpdateCollectionLiteral(ctx *parser.CollectionLiteralContext) bool {
//	// Update collection literals can be one of the following:
//	// * List (not idempotent)
//	// * Set (idempotent)
//	// * Map (idempotent)
//	return ctx.MapLiteral() != nil || ctx.SetLiteral() != nil
//}
//
//func isIdempotentDeleteElementTerm(ctx *parser.TermContext) bool {
//	// Delete element terms can be one of the following:
//	// * Literal (maybe idempotent)
//	// * Bind marker (ambiguous)
//	// * Function call (ambiguous)
//	// * Type cast (ambiguous)
//	return ctx.Literal() != nil && isIdempotentDeleteElementLiteral(ctx.Literal().(*parser.LiteralContext))
//}
//
//func isIdempotentDeleteElementLiteral(ctx *parser.LiteralContext) bool {
//	// Delete element literals can be one of the following:
//	// * Primitive (maybe idempotent)
//	// * Collection (idempotent)
//	// * Tuple (idempotent)
//	// * UDT (idempotent)
//	// * `null` (idempotent, but maybe it's not valid)
//	if ctx.PrimitiveLiteral() != nil {
//		return isIdempotentDeleteElementPrimitiveLiteral(ctx.PrimitiveLiteral().(*parser.PrimitiveLiteralContext))
//	}
//	return true // All other types would be keys for a map, so they'd be idempotent.
//}
//
//func isIdempotentDeleteElementPrimitiveLiteral(ctx *parser.PrimitiveLiteralContext) bool {
//	// Only integers can be used to index lists so this is potentially *NOT* idempotent.
//
//	// The problem this can also be used to remove elements from `set<int>` or `map<int, ...>` and those
//	// operations *ARE* idempotent, but since we don't know the type of the value being indexed we can't
//	// disambiguate these cases from the list case.
//	return ctx.INTEGER() == nil
//}

func extractUnaliasedSelector(ctx *parser.UnaliasedSelectorContext) (interface{}, error) {
	if ctx.K_COUNT() != nil {
		return &CountStarSelector{Name: ctx.GetText()}, nil
	} else if ctx.Identifier() != nil {
		return &IDSelector{
			Name: extractIdentifier(ctx.Identifier().(*parser.IdentifierContext)),
		}, nil
	} else {
		return nil, errors.New("unsupported select clause for system table")
	}
}

func extractIdentifier(cxt *parser.IdentifierContext) string {
	if unquotedIdentifier := cxt.UNQUOTED_IDENTIFIER(); unquotedIdentifier != nil {
		return strings.ToLower(unquotedIdentifier.GetText())
	} else if quotedIdentifier := cxt.QUOTED_IDENTIFIER(); quotedIdentifier != nil {
		identifier := quotedIdentifier.GetText()
		// remove surrounding quotes
		identifier = identifier[1 : len(identifier)-1]
		// handle escaped double-quotes
		identifier = strings.ReplaceAll(identifier, "\"\"", "\"")
		return identifier
	} else {
		return strings.ToLower(cxt.UnreservedKeyword().GetText())
	}
}

func ParseFast(query string) (interface{}, bool) {
	var l Lexer
	var err error
	l.Init(query)

	t := l.Next()
	switch t {
	case TkSelect:
		l.Mark()
		t = l.Next()
		for t != TkFrom && t != TkEOF {
			t = l.Next()
		}
		if t != TkFrom {
			return nil, false
		}

		t = l.Next()
		isTable := isLocalPeers(t)
		if t == TkSystemIdentifier {
			if l.Next() != TkDot {
				return nil, false
			}
			t = l.Next()
			if !isLocalPeers(t) {
				return nil, false
			}
		} else if !isTable {
			return nil, false
		}

		stmt := &SelectStatement{Table: l.Current()}

		l.Rewind()
		t = l.Next()
		for t != TkFrom && t != TkEOF {
			t, err = ParseSelector(&l, t, stmt)
			if err != nil {
				return &ErrorSelectStatement{Err: err}, false
			}
			if t == TkComma {
				t = l.Next()
			}
		}

		return nil, true
	case TkUse:
		return nil, l.Next() == TkIdentifier
	}
	return nil, false
}

func isLocalPeers(t Token) bool {
	return t == TkLocalIdentifier || t == TkPeersIdentifier
}

func ParseSelector(l *Lexer, t Token, stmt *SelectStatement) (Token, error) {
	var selector interface{}
	switch t {
	case TkIdentifier:
		selector = &IDSelector{Name: l.Current()}
	case TkStar:
		stmt.Selectors = append(stmt.Selectors, &StarSelector{})
		return l.Next(), nil
	case TkCount:
		t = l.Next()
		if t == TkStar {
			selector = &CountStarSelector{Name: "COUNT(*)"}
		} else if t == TkIdentifier {
			selector = &CountStarSelector{Name: "COUNT(" + l.Current() + ")"}
		} else {
			return TkInvalid, errors.New("expected * or identifier in argument `COUNT(...)`")
		}
	default:
		return TkInvalid, errors.New("unsupported selector type")
	}

	t = l.Next()
	if t == TkAs {
		t = l.Next()
		if t != TkIdentifier {
			return TkInvalid, errors.New("expected identifier after `AS`")
		}
		stmt.Selectors = append(stmt.Selectors, &AliasSelector{Selector: selector, Alias: l.Current()})
		t = l.Next()
	}

	return t, nil
}
