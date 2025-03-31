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
)

// Determines is the proxy handles the select statement.
//
// Currently, the only handled 'SELECT' queries are for tables in the 'system' keyspace and are matched by the
// `isSystemTable()` function. This includes 'system.local' 'system.peers/peers_v2', and legacy schema tables.
//
// selectStmt: 'SELECT' 'JSON'? 'DISTINCT'? 'FROM' selectClause ...
// selectClause: '*' | selectors
//
// Note: Exclusiveness of '*' not enforced
func isHandledSelectStmt(l *lexer, keyspace Identifier) (handled bool, stmt Statement, err error) {
	l.mark() // Mark here because we might come back to parse the selector
	t := untilToken(l, tkFrom)

	if tkFrom != t {
		return false, nil, errors.New("expected 'FROM' in select statement")
	}

	if t = l.next(); tkIdentifier != t {
		return false, nil, errors.New("expected identifier after 'FROM' in select statement")
	}

	qualifyingKeyspace, table, t, err := parseQualifiedIdentifier(l)
	if err != nil || (!keyspace.equal("system") && !qualifyingKeyspace.equal("system")) || !isSystemTable(table) {
		return false, nil, err
	}

	selectStmt := &SelectStatement{Keyspace: "system", Table: table.id}

	// This only parses the selectors if this is a query handled by the proxy

	l.rewind() // Rewind to the selectors
	for t = l.next(); tkFrom != t && tkEOF != t; t = skipToken(l, t, tkComma) {
		if tkIdentifier == t && (isUnreservedKeyword(l, t, "json") || isUnreservedKeyword(l, t, "distinct")) {
			return true, nil, errors.New("proxy is unable to do 'JSON' or 'DISTINCT' for handled system queries")
		}
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
	return true, &UseStatement{Keyspace: l.identifierStr()}, nil
}

// Parses selectors in the select clause of a select statement.
//
// selectors: selector ( ',' selector )*
// selector: unaliasedSelector ( 'AS' identifier )
// unaliasedSelector:
//
//	identifier
//	'COUNT(*)' | 'COUNT' '(' identifier ')' | NOW()'
//	term
//	'CAST' '(' unaliasedSelector 'AS' primitiveType ')'
//
// Note: Doesn't handle term or cast
func parseSelector(l *lexer, t token) (selector Selector, next token, err error) {
	switch t {
	case tkIdentifier:
		name := l.identifierStr()
		l.mark()
		if tkLparen == l.next() {
			var args []string
			for t = l.next(); tkRparen != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
				if tkStar == t {
					args = append(args, "*")
				} else if tkIdentifier == t {
					args = append(args, l.identifierStr())
				} else {
					return nil, tkInvalid, fmt.Errorf("unexpected argument type for function call '%s(...)' in select statement", name)
				}
			}
			if tkRparen != t {
				return nil, tkInvalid, fmt.Errorf("expected closing ')' for function call '%s' in select statement", name)
			}
			if strings.EqualFold(name, "count") {
				if len(args) == 0 {
					return nil, tkInvalid, fmt.Errorf("expected * or identifier in argument 'COUNT(...)' in select statement")
				}
				return &CountFuncSelector{Arg: args[0]}, l.next(), nil
			} else if strings.EqualFold(name, "now") {
				if len(args) != 0 {
					return nil, tkInvalid, fmt.Errorf("unexpected argument for 'NOW()' function call in select statement")
				}
				return &NowFuncSelector{}, l.next(), nil
			} else {
				return nil, tkInvalid, fmt.Errorf("unsupported function call '%s' in select statement", name)
			}
		} else {
			l.rewind()
			selector = &IDSelector{Name: name}
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
		return &AliasSelector{Selector: selector, Alias: l.identifierStr()}, l.next(), nil
	}

	return selector, t, nil
}