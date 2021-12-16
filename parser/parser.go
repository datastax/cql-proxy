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

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	CountValueName = "count(*)"
)

type parseState uint32

var systemTables = []string{"local", "peers", "peers_v2", "schema_keyspaces", "schema_columnfamilies", "schema_columns", "schema_usertypes"}
var nonIdempotentFuncs = []string{"uuid", "now"}

type Selector interface {
	isSelector()
}

type AliasSelector struct {
	Selector Selector
	Alias    string
}

func (a AliasSelector) isSelector() {}

type IDSelector struct {
	Name string
}

func (I IDSelector) isSelector() {}

type StarSelector struct{}

func (s StarSelector) isSelector() {}

type CountStarSelector struct {
	Name string
}

func (c CountStarSelector) isSelector() {}

type Statement interface {
	isStatement()
}

type SelectStatement struct {
	Table     string
	Selectors []Selector
}

func (s SelectStatement) isStatement() {}

type UseStatement struct {
	Keyspace string
}

func (u UseStatement) isStatement() {}

type ValueLookupFunc func(name string) (value message.Column, err error)

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

func valueFromSelector(selector Selector, valueFunc ValueLookupFunc) (val message.Column, err error) {
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

func isCountSelector(selector Selector) bool {
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

func columnFromSelector(selector Selector, columns []*message.ColumnMetadata, table string) (column *message.ColumnMetadata, err error) {
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

func IsQueryHandled(keyspace string, query string) (handled bool, stmt Statement, err error) {
	var l lexer
	l.init(query)

	t := l.next()
	switch t {
	case tkSelect:
		return isHandledSelectStmt(&l, keyspace)
	case tkUse:
		return isHandledUseStmt(&l)
	}
	return false, nil, nil
}

func IsQueryIdempotent(query string) (idempotent bool, err error) {
	var l lexer
	l.init(query)
	return isIdempotentStmt(&l, l.next())
}

func isHandledSelectStmt(l *lexer, keyspace string) (handled bool, stmt Statement, err error) {
	l.mark()
	t := untilToken(l, tkFrom)

	if tkFrom != t {
		return false, nil, errors.New("expected 'FROM' in select statement")
	}

	if t = l.next(); tkIdentifier != t {
		return false, nil, errors.New("expected identifier after 'FROM' in select statement")
	}

	qualifyingKeyspace, table, t, err := parseQualifiedIdentifier(l)
	if err != nil {
		return false, nil, err
	}
	if (!strings.EqualFold(keyspace, "system") && !strings.EqualFold(qualifyingKeyspace, "system")) || !isSystemTable(table) {
		return false, nil, nil
	}

	selectStmt := &SelectStatement{Table: table}

	l.rewind()
	for t = l.next(); tkFrom != t && tkEOF != t; t = skipToken(l, t, tkComma) {
		var selector Selector
		selector, t, err = parseSelector(l, t)
		if err != nil {
			return true, nil, err
		}
		selectStmt.Selectors = append(selectStmt.Selectors, selector)
	}

	return true, selectStmt, nil
}

func isHandledUseStmt(l *lexer) (handled bool, stmt Statement, err error) {
	t := l.next()
	if tkIdentifier != t {
		return false, nil, errors.New("expected identifier after 'USE' in use statement")
	}
	return true, &UseStatement{Keyspace: l.current()}, nil
}

func isUnreservedKeyword(l *lexer, t token, keyword string) bool {
	return tkIdentifier == t && strings.EqualFold(l.current(), keyword)
}

func parseSelector(l *lexer, t token) (selector Selector, next token, err error) {
	switch t {
	case tkIdentifier:
		if isUnreservedKeyword(l, t, "count") {
			countText := l.current()
			if tkLparen != l.next() {
				return nil, tkInvalid, errors.New("expected '(' after 'COUNT' in select statement")
			}
			if t = l.next(); tkStar == t {
				selector = &CountStarSelector{Name: countText + "(*)"}
			} else if tkIdentifier == t {
				selector = &CountStarSelector{Name: countText + "(" + l.current() + ")"}
			} else {

				return nil, tkInvalid, errors.New("expected * or identifier in argument 'COUNT(...)' in select statement")
			}
			if tkRparen != l.next() {
				return nil, tkInvalid, errors.New("expected closing ')' for 'COUNT' in select statement")
			}
		} else {
			selector = &IDSelector{Name: l.current()}
		}
	case tkStar:
		return &StarSelector{}, l.next(), nil
	default:
		return nil, tkInvalid, errors.New("unsupported select clause for system table")
	}

	if t = l.next(); isUnreservedKeyword(l, t, "as") {
		if tkIdentifier != l.next() {
			return nil, tkInvalid, errors.New("expected identifier after 'AS' in select statement")
		}
		return &AliasSelector{Selector: selector, Alias: l.current()}, l.next(), nil
	}

	return selector, t, nil
}

func isIdempotentStmt(l *lexer, t token) (idempotent bool, err error) {
	switch t {
	case tkSelect:
		return true, nil
	case tkUse:
		return false, nil
	case tkInsert:
		return isIdempotentInsertStmt(l)
	case tkUpdate:
		return isIdempotentUpdateStmt(l)
	case tkDelete:
		return isIdempotentDeleteStmt(l)
	case tkBegin:
		return isIdempotentBatchStmt(l)
	}
	return false, nil
}

func isIdempotentInsertStmt(l *lexer) (idempotent bool, err error) {
	t := l.next()
	if tkInto != t {
		return false, errors.New("expected 'INTO' after 'INSERT' for insert statement")
	}

	if t = l.next(); tkIdentifier != t {
		return false, errors.New("expected identifier after 'INTO' in insert statement")
	}

	_, _, t, err = parseQualifiedIdentifier(l)
	if err != nil {
		return false, err
	}

	if tkLparen != t {
		return false, errors.New("expected '(' after table name for insert statement")
	}

	err = parseIdentifiers(l, l.next())
	if err != nil {
		return false, err
	}

	if !isUnreservedKeyword(l, l.next(), "values") {
		return false, errors.New("expected 'VALUES' after identifiers in insert statement")
	}

	if t != l.next() {
		return false, errors.New("expected '(' after 'VALUES' in insert statement")
	}

	for t = l.next(); tkRparen != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, err
		}
	}

	if t != tkRparen {
		return false, errors.New("expected closing ')' for 'VALUES' list in insert statement")
	}

	for t = l.next(); t != tkEOF; {
		if tkIf == t {
			return false, nil
		}
	}

	return true, nil
}

func isIdempotentUpdateStmt(l *lexer) (idempotent bool, err error) {
	t := l.next()
	if tkIdentifier != t {
		return false, errors.New("expected identifier after 'UPDATE' in update statement")
	}

	_, _, t, err = parseQualifiedIdentifier(l)
	if err != nil {
		return false, err
	}

	t, err = parseUsingClause(l, t)
	if err != nil {
		return false, err
	}

	for !isUnreservedKeyword(l, t, "set") {
		return false, errors.New("expected 'SET' in update statement")
	}

	for t = l.next(); tkIf != t && tkWhere != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
		idempotent, err = parseUpdateOp(l, t)
		if !idempotent {
			return idempotent, err
		}
	}

	if tkWhere == t {
		idempotent, t, err = parseWhereClause(l)
		if !idempotent {
			return idempotent, err
		}
	}

	for tkIf != t && tkEOF != t {
		t = l.next()
	}

	if tkIf == t {
		return false, nil
	}

	return true, nil
}

func parseUsingClause(l *lexer, t token) (next token, err error) {
	if tkUsing == t {
		err = parseTtlOrTimestamp(l)
		if err != nil {
			return tkInvalid, err
		}
		if t = l.next(); tkAnd == t {
			err = parseTtlOrTimestamp(l)
			if err != nil {
				return tkInvalid, err
			}
		}
	}
	return t, nil
}

func parseTtlOrTimestamp(l *lexer) error {
	var t token
	if t = l.next(); !isUnreservedKeyword(l, t, "ttl") && !isUnreservedKeyword(l, t, "timestamp") {
		return errors.New("expected 'TTL' or 'TIMESTAMP' after 'USING'")
	}
	switch t = l.next(); t {
	case tkInteger:
		return nil
	case tkColon, tkQMark:
		return parseBindMarker(l, t)
	}
	return errors.New("expected integer or bind marker after 'TTL' or 'TIMESTAMP'")
}

func parseWhereClause(l *lexer) (idempotent bool, t token, err error) {
	for t = l.next(); tkIf != t && tkEOF != t; t = skipToken(l, l.next(), tkAnd) {
		idempotent, err = parseRelation(l, t)
		if !idempotent {
			return idempotent, tkInvalid, err
		}
	}
	return true, t, nil
}

func parseRelation(l *lexer, t token) (idempotent bool, err error) {
	switch t {
	case tkIdentifier:
		switch t = l.next(); t {
		case tkIdentifier:
			if isUnreservedKeyword(l, t, "contains") { // identifier 'contains' 'key'? term
				if t = l.next(); isUnreservedKeyword(l, t, "key") {
					t = l.next()
				}
				if idempotent, _, err = parseTerm(l, t); !idempotent {
					return idempotent, err
				}

			} else if isUnreservedKeyword(l, t, "like") { // identifier 'like' term
				if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
					return idempotent, err
				}
			} else {
				return false, errors.New("unexpected token parsing relation")
			}
		case tkEqual, tkRangle, tkLtEqual, tkLangle, tkGtEqual, tkNotEqual: // identifier operator term
			if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, err
			}
		case tkIs: // identifier 'is' 'not' 'null'
			if t = l.next(); tkNot != t {
				return false, errors.New("expected 'not' in relation after 'is'")
			}
			if t = l.next(); tkNull != t {
				return false, errors.New("expected 'null' in relation after 'is not'")
			}
		case tkLsquare: // identifier '[' term ']' operator term
			if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, err
			}
			if t = l.next(); tkRsquare != t {
				return false, errors.New("expected closing ']' after term in relation")
			}
			if t = l.next(); !isOperator(t) {
				return false, errors.New("expected operator after term in relation")
			}
			if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, err
			}
		case tkIn: // identifier 'in' ('(' terms? ')' | bindMarker)
			switch t = l.next(); t {
			case tkLparen:
				for t = l.next(); tkRparen != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
					if idempotent, _, err = parseTerm(l, t); !idempotent {
						return idempotent, err
					}
				}
			case tkColon, tkQMark:
				err = parseBindMarker(l, t)
				if err != nil {
					return false, err
				}
			default:
				return false, errors.New("unexpected token for 'IN' relation")
			}
		default:
			return false, errors.New("unexpected token parsing relation")
		}
	case tkToken: // token '(' identifiers ')' operator term
		if t = l.next(); tkLparen != t {
			return false, errors.New("expected '(' after 'token'")
		}
		err = parseIdentifiers(l, l.next())
		if err != nil {
			return false, err
		}
		if t = l.next(); !isOperator(t) {
			return false, errors.New("expected operator after identifier list in relation")
		}
		if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
			return idempotent, err
		}
	case tkLparen: // '(' relation ')' | '(' identifiers ')' ...
		l.mark()
		maybeId, maybeCommaOrRparen := l.next(), l.next() // Peek a couple tokens to see if this is an identifier list
		if tkIdentifier == maybeId && (maybeCommaOrRparen == tkComma || maybeCommaOrRparen == tkRparen) {
			t = skipToken(l, maybeCommaOrRparen, tkComma)
			err = parseIdentifiers(l, t)
			if err != nil {
				return false, err
			}
			return parseIdentifiersRelation(l)
		} else {
			l.rewind()
			idempotent, err = parseRelation(l, l.next())
			if !idempotent {
				return idempotent, err
			}
			if tkRparen != l.next() {
				return false, errors.New("expected closing ')' after parenthesized relation")
			}
		}
	default:
		return false, errors.New("unexpected token in relation")
	}
	return true, nil
}

func parseIdentifiersRelation(l *lexer) (idempotent bool, err error) {
	switch t := l.next(); t {
	case tkIn, tkEqual, tkLt, tkLtEqual, tkGt, tkGtEqual, tkNotEqual: // '(' identifiers ')' 'in' ... | '(' identifiers ')' operator ...
		switch t = l.next(); t {
		case tkColon, tkQMark:
			err = parseBindMarker(l, t)
			if err != nil {
				return false, err
			}
		case tkLparen:
			for t = l.next(); tkRparen != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
				if idempotent, _, err = parseTerm(l, t); !idempotent {
					return idempotent, err
				}
			}
			if tkRparen != t {
				return false, errors.New("expected closing ')' in identifiers relation")
			}
		}
	default:
		return false, errors.New("unexpected token in identifiers relation")
	}

	return true, nil
}

func parseBindMarker(l *lexer, t token) error {
	switch t {
	case tkColon:
		if tkIdentifier != l.next() {
			return errors.New("expected identifier after ':' for named bind marker")
		}
	case tkQMark:
		// Do nothing
	default:
		return errors.New("invalid bind marker")
	}
	return nil
}

func parseIdentifiers(l *lexer, t token) (err error) {
	for tkRparen != t && tkEOF != t {
		if tkIdentifier != t {
			return errors.New("expected identifier")
		}
		t = skipToken(l, l.next(), tkComma)
	}
	if tkRparen != t {
		return errors.New("expected closing ')' for identifiers")
	}
	return nil
}

func isOperator(t token) bool {
	return tkEqual == t || tkLt == t || tkLtEqual == t || tkGt == t || tkGtEqual == t || tkNotEqual == t
}

func parseUpdateOp(l *lexer, t token) (idempotent bool, err error) {
	if tkIdentifier != t {
		return false, errors.New("expected identifier after 'SET' in update statement")
	}

	var typ termType

	switch t = l.next(); t {
	case tkEqual:
		l.mark()
		maybeId, maybeAddOrSub := l.next(), l.next()
		if tkIdentifier == maybeId && (tkAdd == maybeAddOrSub || tkSub == maybeAddOrSub) { // identifier = identifier + term | identifier = identifier - term
			t = l.next()
			if idempotent, typ, err = parseTerm(l, t); !idempotent {
				return idempotent, err
			}
			return isIdempotentUpdateOpTermType(typ), nil

		} else {
			l.rewind()
			t = l.next()
			if idempotent, typ, err = parseTerm(l, t); idempotent { // identifier = term | identifier = term + identifier
				l.mark()
				if t = l.next(); tkAdd == t {
					if tkIdentifier != l.next() {
						return false, errors.New("expected identifier after '+' operator in update operation")
					}
					return isIdempotentUpdateOpTermType(typ), nil
				} else {
					l.rewind()
				}
			} else {
				return idempotent, err
			}
		}
	case tkAddEqual, tkSubEqual: // identifier += term | identifier -= term
		t = l.next()
		if idempotent, typ, err = parseTerm(l, t); !idempotent {
			return idempotent, err
		}
		return isIdempotentUpdateOpTermType(typ), nil
	case tkLsquare: // identifier '[' term ']' = term
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, err
		}
		if tkRsquare != l.next() {
			return false, errors.New("expected closing ']' in update operation")
		}
		if tkEqual != l.next() {
			return false, errors.New("expected '=' in update operation")
		}
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, err
		}
	case tkDot: // identifier '.' identifier '=' term
		if t = l.next(); tkIdentifier != t {
			return false, errors.New("expected identifier after '+' operator in update operation")
		}
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, err
		}
	default:
		return false, errors.New("unexpected token in update operation")
	}

	return true, nil
}

func isIdempotentUpdateOpTermType(typ termType) bool {
	// Update terms can be one of the following:
	// * Literal (idempotent, if not a list)
	// * Bind marker (ambiguous, so not idempotent)
	// * Function call (ambiguous, so not idempotent)
	// * Type cast (probably not idempotent)
	return typ == termSetMapUdtLiteral || typ == termTupleLiteral
}

func isIdempotentDeleteElementTermType(typ termType) bool {
	// Delete element terms can be one of the following:
	// * Literal (idempotent, if not an integer literal)
	// * Bind marker (ambiguous, so not idempotent)
	// * Function call (ambiguous, so not idempotent)
	// * Type cast (ambiguous)
	return typ != termIntegerLiteral && typ != termBindMarker && typ != termFunctionCall && typ != termCast
}

func isIdempotentDeleteStmt(l *lexer) (idempotent bool, err error) {
	var t token
	for t = l.next(); tkFrom != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
		if tkIdentifier != t {
			return false, errors.New("unexpected token after 'DELETE' in delete statement")
		}

		l.mark()
		switch t = l.next(); t {
		case tkLsquare:
			var typ termType
			t = l.next()
			if idempotent, typ, err = parseTerm(l, t); !idempotent {
				return idempotent, err
			}
			if tkRsquare != l.next() {
				return false, errors.New("expected closing ']' for the delete operation")
			}
			return isIdempotentDeleteElementTermType(typ), nil
		case tkDot:
			if tkIdentifier != l.next() {
				return false, errors.New("expected another identifier after '.' for delete operation")
			}
		default:
			l.rewind()
		}
	}

	for tkIf != t && tkWhere != t && tkEOF != t {
		t = l.next()
	}

	if tkWhere == t {
		idempotent, t, err = parseWhereClause(l)
		if !idempotent {
			return idempotent, err
		}
	}

	for tkIf != t && tkEOF != t {
		t = l.next()
	}

	if tkIf == t {
		return false, nil
	}

	return true, nil
}

func isIdempotentBatchStmt(l *lexer) (idempotent bool, err error) {
	return false, nil
}

type termType int

const (
	termInvalid        termType = iota
	termIntegerLiteral          // Special because it can be used to index lists for deletes
	termPrimitiveLiteral
	termListLiteral
	termSetMapUdtLiteral // All use curly, distinction not important
	termTupleLiteral
	termBindMarker
	termFunctionCall
	termCast
)

func parseQualifiedIdentifier(l *lexer) (keyspace, target string, t token, err error) {
	temp := l.current()
	if t = l.next(); tkDot == t {
		if t = l.next(); tkIdentifier != t {
			return "", "", tkInvalid, errors.New("expected another identifier after '.' for qualified identifier")
		}
		return temp, l.current(), l.next(), nil
	} else {
		return "", temp, t, nil
	}
}

func parseType(l *lexer) (t token, err error) {
	if t = l.next(); tkLangle == t {
		for t = l.next(); tkRangle != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
			if t != tkIdentifier {
				return tkInvalid, errors.New("expected sub-type in type parameter")
			}
		}
		if tkRangle != t {
			return tkInvalid, errors.New("expected closing '>' bracket for type")
		}
		return l.next(), nil
	}
	return t, nil
}

func parseTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	switch t {
	case tkInteger:
		return true, termIntegerLiteral, nil
	case tkFloat, tkBool, tkNull, tkStringLiteral, tkHexNumber, tkUuid, tkDuration, tkNan, tkInfinity: // Literal
		return true, termPrimitiveLiteral, nil
	case tkLsquare: // List literal
		for t = l.next(); t != tkRsquare && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, termInvalid, err
			}
		}
		if t != tkRsquare {
			return false, termInvalid, errors.New("expected closing ']' bracket for list literal")
		}
		return true, termListLiteral, nil
	case tkLcurly: // Set, map, or UDT literal
		if t = l.next(); t == tkIdentifier { // UDT
			for t != tkRcurly && t != tkEOF {
				_, _, t, err = parseQualifiedIdentifier(l)
				if err != nil {
					return false, termInvalid, err
				}
				t = skipToken(l, l.next(), tkColon)
				if idempotent, typ, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				t = skipToken(l, l.next(), tkComma)
			}
		} else {
			for t != tkRcurly && t != tkEOF {
				if idempotent, typ, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				if t = l.next(); tkColon == t { // Map
					if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
						return idempotent, termInvalid, err
					}
					t = l.next()
				}
				t = skipToken(l, t, tkComma)
			}
		}
		if t != tkRcurly {
			return false, termInvalid, errors.New("expected closing '}' bracket for set/map/UDT literal")
		}
		return true, termSetMapUdtLiteral, nil
	case tkLparen: // Type cast or tuple literal
		if t = l.next(); t == tkIdentifier { // Cast
			t, err = parseType(l)
			if err != nil {
				return false, termInvalid, err
			}
			if t != tkRparen {
				return false, termInvalid, errors.New("expected closing ')' bracket for type cast")
			}
			if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, termInvalid, err
			}
			return true, termCast, err
		} else { // Tuple literal
			for t != tkRparen && t != tkEOF {
				if idempotent, _, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				t = skipToken(l, l.next(), tkComma)
			}
			if t != tkRparen {
				return false, termInvalid, errors.New("expected closing ')' bracket for tuple literal")
			}
			return true, termTupleLiteral, nil
		}
	case tkColon: // Named bind marker
		if t = l.next(); t != tkIdentifier {
			return false, termInvalid, errors.New("expected identifier after ':' for named bind marker")
		}
		return true, termBindMarker, nil
	case tkQMark: // Positional bind marker
		return true, termBindMarker, nil
	case tkIdentifier: // Function
		var target, keyspace string
		keyspace, target, t, err = parseQualifiedIdentifier(l)
		if err != nil {
			return false, termInvalid, err
		}
		if tkLparen != t {
			return false, termInvalid, errors.New("invalid term, was expecting function call")
		}
		for t = l.next(); t != tkRparen && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, termInvalid, err
			}
		}
		if t != tkRparen {
			return false, termInvalid, errors.New("expected closing ')' for function call")
		}
		return !(isNonIdempotentFunc(target) && (len(keyspace) == 0 || strings.EqualFold("system", keyspace))), termFunctionCall, nil
	}

	return false, termInvalid, errors.New("invalid term")
}

func skipToken(l *lexer, t token, toSkip token) token {
	if t == toSkip {
		return l.next()
	}
	return t
}

func untilToken(l *lexer, to token) token {
	var t token
	for to != t && tkEOF != t {
		t = l.next()
	}
	return t
}
