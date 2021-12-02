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
	"errors"
	"fmt"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	parser "github.com/datastax/cql-proxy/proxycore/antlr"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	CountValueName = "count(*)"
)

type parseContext uint32

const (
	inSelectStatement parseContext = 1 << iota
	inSelectCause
	inInsertStatement
	inTerms
	inFunctionName
	inUpdateStatement
	inUpdateOperations
	inUpdateOperationAddLeft
	inUpdateOperationAddRight
	inUpdateOperationSubtract
	inUpdateOperationAddAssign
	inUpdateOperationSubtractAssign
	inDeleteStatement
	inDeleteOperationElement
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
	parseCtx   parseContext
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

func (l *queryListener) EnterInsertStatement(ctx *parser.InsertStatementContext) {
	l.idempotent = true
	l.parseCtx |= inInsertStatement

	if ctx.K_IF() != nil {
		l.idempotent = false
	}
}

func (l *queryListener) ExitInsertStatement(_ *parser.InsertStatementContext) {
	l.parseCtx &= ^inInsertStatement
}

func (l *queryListener) EnterUpdateStatement(ctx *parser.UpdateStatementContext) {
	l.idempotent = true
	l.parseCtx |= inUpdateStatement

	if ctx.K_IF() != nil {
		l.idempotent = false
	}
}

func (l *queryListener) ExitUpdateStatement(_ *parser.UpdateStatementContext) {
	l.parseCtx &= ^inUpdateStatement
}

func (l *queryListener) EnterDeleteStatement(ctx *parser.DeleteStatementContext) {
	l.idempotent = true
	l.parseCtx |= inDeleteStatement

	if ctx.K_IF() != nil {
		l.idempotent = false
	}
}

func (l *queryListener) EnterUpdateOperations(_ *parser.UpdateOperationsContext) {
	l.parseCtx |= inUpdateOperations
}

func (l *queryListener) ExitUpdateOperations(_ *parser.UpdateOperationsContext) {
	l.parseCtx &= ^inUpdateOperations
}

func (l *queryListener) EnterUpdateOperatorAddLeft(_ *parser.UpdateOperatorAddLeftContext) {
	l.parseCtx |= inUpdateOperationAddLeft
}

func (l *queryListener) ExitUpdateOperatorAddLeft(_ *parser.UpdateOperatorAddLeftContext) {
	l.parseCtx &= ^inUpdateOperationAddLeft
}

func (l *queryListener) EnterUpdateOperatorAddRight(_ *parser.UpdateOperatorAddRightContext) {
	l.parseCtx |= inUpdateOperationAddRight
}

func (l *queryListener) ExitUpdateOperatorAddRight(_ *parser.UpdateOperatorAddRightContext) {
	l.parseCtx &= ^inUpdateOperationAddRight
}

func (l *queryListener) EnterUpdateOperatorSubtract(_ *parser.UpdateOperatorSubtractContext) {
	l.parseCtx |= inUpdateOperationSubtract
}

func (l *queryListener) ExitUpdateOperatorSubtract(_ *parser.UpdateOperatorSubtractContext) {
	l.parseCtx &= ^inUpdateOperationSubtract
}

func (l *queryListener) EnterUpdateOperatorAddAssign(_ *parser.UpdateOperatorAddAssignContext) {
	l.parseCtx |= inUpdateOperationAddAssign
}

func (l *queryListener) ExitUpdateOperatorAddAssign(_ *parser.UpdateOperatorAddAssignContext) {
	l.parseCtx &= ^inUpdateOperationAddAssign
}

func (l *queryListener) EnterUpdateOperatorSubtractAssign(_ *parser.UpdateOperatorSubtractAssignContext) {
	l.parseCtx |= inUpdateOperationSubtractAssign
}

func (l *queryListener) ExitUpdateOperatorSubtractAssign(_ *parser.UpdateOperatorSubtractAssignContext) {
	l.parseCtx &= ^inUpdateOperationSubtractAssign
}


func (l *queryListener) EnterDeleteOperationElement(_ *parser.DeleteOperationElementContext) {
	l.parseCtx |= inDeleteOperationElement
}

func (l *queryListener) ExitDeleteOperationElement(_ *parser.DeleteOperationElementContext) {
	l.parseCtx &= ^inDeleteOperationElement
}

func (l *queryListener) EnterListLiteral(_ *parser.ListLiteralContext) {
	// Queries that prepend or append to a list are *NOT* idempotent
	if l.parseCtx&(inUpdateStatement|inUpdateOperations|inUpdateOperationAddLeft|inUpdateOperationAddRight|inUpdateOperationAddAssign) > 0 {
		l.idempotent = false
	}
}

func (l *queryListener) EnterPrimitiveLiteral(ctx *parser.PrimitiveLiteralContext) {
	// Queries that modify counters are *NOT* idempotent
	if ctx.INTEGER() != nil && l.parseCtx&(inUpdateStatement|inUpdateOperations|inUpdateOperationAddLeft|inUpdateOperationAddRight|
		inUpdateOperationAddAssign|inUpdateOperationSubtract|inUpdateOperationSubtractAssign) > 0 {
		l.idempotent = false
	}
	// Queries that delete from lists are *NOT* idempotent.
	// TODO: This needs to be disambiguated from `set<int>` deletes
	if ctx.INTEGER() != nil && l.parseCtx&(inDeleteStatement|inDeleteOperationElement) > 0 {
		l.idempotent = false
	}
}

func (l *queryListener) ExitDeleteStatement(_ *parser.DeleteStatementContext) {
	l.parseCtx &= ^inDeleteStatement
}

func (l *queryListener) EnterTerms(_ *parser.TermsContext) {
	l.parseCtx |= inTerms
}

func (l *queryListener) ExitTerms(_ *parser.TermsContext) {
	l.parseCtx &= ^inTerms
}

func (l *queryListener) EnterFunctionName(_ *parser.FunctionNameContext) {
	l.parseCtx |= inFunctionName
}

func (l *queryListener) ExitFunctionName(_ *parser.FunctionNameContext) {
	l.parseCtx &= ^inFunctionName
}

func (l *queryListener) EnterIdentifier(ctx *parser.IdentifierContext) {
	// Queries that use the functions `uuid()` or `now()` in writes are *NOT* idempotent
	if (l.parseCtx&(inInsertStatement|inTerms|inFunctionName) > 0) ||
		(l.parseCtx&(inUpdateStatement|inUpdateOperations|inFunctionName) > 0) {
		funcName := strings.ToLower(extractIdentifier(ctx))
		if funcName == "uuid" || funcName == "now" {
			l.idempotent = false
		}
	}
}

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
