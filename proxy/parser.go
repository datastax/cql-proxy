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

package proxy

import (
	parser "cql-proxy/proxycore/antlr"
	"errors"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"strings"
)

const (
	countValueName = "count(*)"
)

var systemTables = []string{"local", "peers", "peers_v2"}

type aliasSelector struct {
	selector interface{}
	alias    string
}

type idSelector struct {
	name string
}

type starSelector struct{}

type countStarSelector struct {
	name string
}

type errorSelectStatement struct {
	err error
}

type selectStatement struct {
	table     string
	selectors []interface{}
}

type useStatement struct {
	keyspace string
}

type valueLookupFunc func(name string) (value message.Column, err error)

func parse(keyspace string, query string) (handled bool, idempotent bool, stmt interface{}) {
	is := antlr.NewInputStream(query)
	lexer := parser.NewSimplifiedCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	cqlParser := parser.NewSimplifiedCqlParser(stream)
	listener := &queryListener{keyspace: keyspace}
	antlr.ParseTreeWalkerDefault.Walk(listener, cqlParser.CqlStatement())
	return listener.handled, listener.idempotent, listener.stmt
}

func filterValues(stmt *selectStatement, columns []*message.ColumnMetadata, valueFunc valueLookupFunc) (filtered []message.Column, err error) {
	if _, ok := stmt.selectors[0].(*starSelector); ok {
		for _, column := range columns {
			var val message.Column
			val, err = valueFunc(column.Name)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, val)
		}
	} else {
		for _, selector := range stmt.selectors {
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

func valueFromSelector(selector interface{}, valueFunc valueLookupFunc) (val message.Column, err error) {
	switch s := selector.(type) {
	case *countStarSelector:
		return valueFunc(countValueName)
	case *idSelector:
		return valueFunc(s.name)
	case *aliasSelector:
		return valueFromSelector(s.selector, valueFunc)
	default:
		return nil, errors.New("unhandled selector type")
	}
}

func filterColumns(stmt *selectStatement, columns []*message.ColumnMetadata) (filtered []*message.ColumnMetadata, err error) {
	if _, ok := stmt.selectors[0].(*starSelector); ok {
		filtered = columns
	} else {
		for _, selector := range stmt.selectors {
			var column *message.ColumnMetadata
			column, err = columnFromSelector(selector, columns, stmt.table)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, column)
		}
	}
	return filtered, nil
}

func isCountSelector(selector interface{}) bool {
	_, ok := selector.(*countStarSelector)
	return ok
}

func isCountStarQuery(stmt *selectStatement) bool {
	if len(stmt.selectors) == 1 {
		if isCountSelector(stmt.selectors[0]) {
			return true
		} else if alias, ok := stmt.selectors[0].(*aliasSelector); ok {
			return isCountSelector(alias.selector)
		}
	}
	return false
}

func columnFromSelector(selector interface{}, columns []*message.ColumnMetadata, table string) (column *message.ColumnMetadata, err error) {
	switch s := selector.(type) {
	case *countStarSelector:
		return &message.ColumnMetadata{
			Keyspace: "system",
			Table:    table,
			Name:     s.name,
			Type:     datatype.Int,
		}, nil
	case *idSelector:
		if column = findColumnMetadata(columns, s.name); column != nil {
			return column, nil
		} else {
			return nil, fmt.Errorf("invalid column %s", s.name)
		}
	case *aliasSelector:
		column, err = columnFromSelector(s.selector, columns, table)
		if err != nil {
			return nil, err
		}
		alias := *column // Make a copy so we can modify the name
		alias.Name = s.alias
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

		selectStmt := &selectStatement{
			table:     table,
			selectors: make([]interface{}, 0),
		}

		selectClauseCtx := ctx.SelectClause().(*parser.SelectClauseContext)

		if selectClauseCtx.Selectors() != nil {
			selectorsCtx := selectClauseCtx.Selectors().(*parser.SelectorsContext)
			for _, selector := range selectorsCtx.AllSelector() {
				selectorCtx := selector.(*parser.SelectorContext)
				unaliasedSelector, err := extractUnaliasedSelector(selectorCtx.UnaliasedSelector().(*parser.UnaliasedSelectorContext))
				if err != nil {
					l.stmt = &errorSelectStatement{err}
					return // invalid selector
				}
				if selectorCtx.K_AS() != nil { // alias
					selectStmt.selectors = append(selectStmt.selectors, &aliasSelector{
						selector: unaliasedSelector,
						alias:    extractIdentifier(selectorCtx.Identifier().(*parser.IdentifierContext)),
					})
				} else {
					selectStmt.selectors = append(selectStmt.selectors, unaliasedSelector)
				}
			}
		} else { // 'SELECT * ...'
			selectStmt.selectors = append(selectStmt.selectors, &starSelector{})
		}

		l.stmt = selectStmt
	}
}

func (l *queryListener) EnterUseStatement(ctx *parser.UseStatementContext) {
	l.handled = true
	l.stmt = &useStatement{keyspace: extractIdentifier(ctx.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))}
}

func (l *queryListener) EnterInsertStatement(_ *parser.InsertStatementContext) {
	// TODO: Check is idempotent
}

func (l *queryListener) EnterUpdateStatement(_ *parser.UpdateStatementContext) {
	// TODO: Check is idempotent
}

func (l *queryListener) EnterDeleteStatement(_ *parser.DeleteStatementContext) {
	// TODO: Check is idempotent
}

func extractUnaliasedSelector(selectorCtx *parser.UnaliasedSelectorContext) (interface{}, error) {
	if selectorCtx.K_COUNT() != nil {
		return &countStarSelector{name: selectorCtx.GetText()}, nil
	} else if selectorCtx.Identifier() != nil {
		return &idSelector{
			name: extractIdentifier(selectorCtx.Identifier().(*parser.IdentifierContext)),
		}, nil
	} else {
		return nil, errors.New("unsupported select clause for system table")
	}
}

func extractIdentifier(identifierCxt *parser.IdentifierContext) string {
	if unquotedIdentifier := identifierCxt.UNQUOTED_IDENTIFIER(); unquotedIdentifier != nil {
		return strings.ToLower(unquotedIdentifier.GetText())
	} else if quotedIdentifier := identifierCxt.QUOTED_IDENTIFIER(); quotedIdentifier != nil {
		identifier := quotedIdentifier.GetText()
		// remove surrounding quotes
		identifier = identifier[1 : len(identifier)-1]
		// handle escaped double-quotes
		identifier = strings.ReplaceAll(identifier, "\"\"", "\"")
		return identifier
	} else {
		return strings.ToLower(identifierCxt.UnreservedKeyword().GetText())
	}
}
