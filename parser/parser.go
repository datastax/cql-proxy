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
var nonIdempotentFuncs = []string{"uuid", "now"}

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
	lex := parser.NewSimplifiedCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lex, antlr.TokenDefaultChannel)
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
		if strings.EqualFold(table, name) {
			return true
		}
	}
	return false
}

func isNonIdempotentFunc(name string) bool {
	for _, funcName := range nonIdempotentFuncs {
		if strings.EqualFold(funcName, name) {
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

func IsQueryHandled(query string) (handled bool, stmt interface{}) {
	var l lexer
	l.init(query)

	t := l.next()
	switch t {
	case tkSelect:
		return isHandledSelectStmt(&l)
	case tkUse:
		return isHandledUseStmt(&l)
	}
	return false, nil
}

func IsQueryIdempotent(query string) bool {
	var l lexer
	l.init(query)
	return isIdempotentStmt(&l, l.next())
}

func isIdempotentStmt(l *lexer, t token) bool {
	switch t {
	case tkSelect:
		return true
	case tkUse:
		return false
	case tkInsert:
		return isIdempotentInsertStmt(l)
	case tkUpdate:
		return isIdempotentUpdateStmt(l)
	case tkDelete:
		return isIdempotentDeleteStmt(l)
	case tkBegin:
		return isIdempotentBatchStmt(l)
	}
	return false
}

func isIdempotentInsertStmt(l *lexer) bool {
	if tkInto != l.next() {
		return false
	}

	if tkIdentifier != l.next() {
		return false
	}

	if tkDot == l.next() {
		if tkIdentifier != l.next() {
			return false
		}
	}

	if tkLparen != l.next() {
		return false
	}

	t := l.next()
	for t != tkRparen && t != tkEOF {
		t = l.next()
	}

	if tkRparen != t {
		return false
	}

	if tkValues != l.next() {
		return false
	}

	if tkLparen != l.next() {
		return false
	}

	t = l.next()
	for tkRparen != t && tkEOF != t {
		if !isIdempotentInsertTerm(l, t) {
			return false
		}
		t = l.next()
		if tkComma == t {
			t = l.next()
		}
	}

	if t != tkRparen {
		return false
	}

	if tkIf == l.next() {
		return false
	}

	return true
}

func isIdempotentInsertTerm(l *lexer, t token) bool {
	switch t {
	case tkIdentifier:
		t = l.next()
		if tkDot == t {
			t = l.next()
			if tkIdentifier != t || strings.EqualFold(l.current(), "system") {
				return false
			}
		}

		if tkLparen != l.next() {
			return false
		}

		if tkIdentifier != l.next() {
			return false
		}

		if isNonIdempotentFunc(l.current()) {
			return false
		}

		switch l.next() {
		case tkDot:

		}
	}
	return true
}

func isIdempotentUpdateStmt(l *lexer) bool {
	return false
}

func isIdempotentDeleteStmt(l *lexer) bool {
	return false
}

func isIdempotentBatchStmt(l *lexer) bool {
	return false
}

func isHandledSelectStmt(l *lexer) (handled bool, stmt interface{}) {
	l.mark()
	t := l.next()
	for t != tkFrom && t != tkEOF {
		t = l.next()
	}
	if t != tkFrom {
		return false, nil
	}

	t = l.next()
	if t == tkIdentifier && strings.EqualFold(l.current(), "system") {
		if l.next() != tkDot {
			return false, nil
		}
		t = l.next()
		if !isSystemTable(l.current()) {
			return false, nil
		}
	} else if !isSystemTable(l.current()) {
		return false, nil
	}

	selectStmt := &SelectStatement{Table: l.current()}

	l.rewind()
	t = l.next()
	for t != tkFrom && t != tkEOF {
		var err error
		t, err = parseSelector(l, t, selectStmt)
		if err != nil {
			return true, &ErrorSelectStatement{Err: err}
		}
		if t == tkComma {
			t = l.next()
		}
	}

	return true, stmt
}

func isHandledUseStmt(l *lexer) (handled bool, stmt interface{}) {
	t := l.next()
	if t != tkIdentifier {
		return false, nil
	}
	return true, &UseStatement{Keyspace: l.current()}
}

func parseSelector(l *lexer, t token, stmt *SelectStatement) (token, error) {
	var selector interface{}
	switch t {
	case tkIdentifier:
		selector = &IDSelector{Name: l.current()}
	case tkStar:
		stmt.Selectors = append(stmt.Selectors, &StarSelector{})
		return l.next(), nil
	case tkCount:
		t = l.next()
		if t == tkStar {
			selector = &CountStarSelector{Name: "COUNT(*)"}
		} else if t == tkIdentifier {
			selector = &CountStarSelector{Name: "COUNT(" + l.current() + ")"}
		} else {
			return tkInvalid, errors.New("expected * or identifier in argument `COUNT(...)`")
		}
	default:
		return tkInvalid, errors.New("unsupported selector type")
	}

	t = l.next()
	if t == tkAs {
		t = l.next()
		if t != tkIdentifier {
			return tkInvalid, errors.New("expected identifier after `AS`")
		}
		stmt.Selectors = append(stmt.Selectors, &AliasSelector{Selector: selector, Alias: l.current()})
		t = l.next()
	}

	return t, nil
}
