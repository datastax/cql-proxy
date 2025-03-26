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

	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	CountValueName = "count(*)"
)

var systemTables = []string{"local", "peers", "peers_v2", "schema_keyspaces", "schema_columnfamilies", "schema_columns", "schema_usertypes"}

var nonIdempotentFuncs = []string{"uuid", "now"}

type ValueLookupFunc func(name string) (value message.Column, err error)

func FilterValues(stmt *SelectStatement, columns []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	for _, selector := range stmt.Selectors {
		var vals []message.Column
		vals, err = selector.Values(columns, valueFunc)
		if err != nil {
			return nil, err
		}
		filtered = append(filtered, vals...)
	}
	return filtered, nil
}

func FilterColumns(stmt *SelectStatement, columns []*message.ColumnMetadata) (filtered []*message.ColumnMetadata, err error) {
	for _, selector := range stmt.Selectors {
		var columns []*message.ColumnMetadata
		columns, err = selector.Columns(columns, stmt)
		if err != nil {
			return nil, err
		}
		filtered = append(filtered, columns...)
	}
	return filtered, nil
}

func isSystemTable(name Identifier) bool {
	for _, table := range systemTables {
		if name.equal(table) {
			return true
		}
	}
	return false
}

func isNonIdempotentFunc(name Identifier) bool {
	for _, funcName := range nonIdempotentFuncs {
		if name.equal(funcName) {
			return true
		}
	}
	return false
}

func isUnreservedKeyword(l *lexer, t token, keyword string) bool {
	return tkIdentifier == t && l.identifier().equal(keyword)
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

func parseQualifiedIdentifier(l *lexer) (keyspace, target Identifier, t token, err error) {
	temp := l.identifier()
	if t = l.next(); tkDot == t {
		if t = l.next(); tkIdentifier != t {
			return Identifier{}, Identifier{}, tkInvalid, errors.New("expected another identifier after '.' for qualified identifier")
		}
		return temp, l.identifier(), l.next(), nil
	} else {
		return Identifier{}, temp, t, nil
	}
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

func isDMLTerminator(t token) bool {
	return t == tkEOF || t == tkEOS || t == tkInsert || t == tkUpdate || t == tkDelete || t == tkApply
}
