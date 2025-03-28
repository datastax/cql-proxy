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
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	CountValueName = "count(*)"
)

var systemTables = []string{"local", "peers", "peers_v2", "schema_keyspaces", "schema_columnfamilies", "schema_columns", "schema_usertypes"}

var nonIdempotentFuncs = []string{"uuid", "now"}

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
			column, err = columnFromSelector(selector, columns, stmt.Keyspace, stmt.Table)
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

func columnFromSelector(selector Selector, columns []*message.ColumnMetadata, keyspace string, table string) (column *message.ColumnMetadata, err error) {
	switch s := selector.(type) {
	case *CountStarSelector:
		return &message.ColumnMetadata{
			Keyspace: keyspace,
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
		column, err = columnFromSelector(s.Selector, columns, keyspace, table)
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

// PatchQueryConsistency modifies the consistency level of a QUERY message in-place
// by locating the consistency field directly in the frame body.
//
// Layout based on the CQL native protocol v4 spec:
// /* <query: long string><consistency: short><flags: byte>... */
func PatchQueryConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	if len(body) < 6 {
		return fmt.Errorf("body too short for QUERY")
	}
	queryLen := binary.BigEndian.Uint32(body[0:4])
	offset := 4 + int(queryLen)
	if len(body) < offset+2 {
		return fmt.Errorf("not enough bytes to patch QUERY consistency")
	}
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(newConsistency))
	return nil
}

// PatchExecuteConsistency modifies the consistency level of an EXECUTE message in-place
// by locating the consistency field directly after the prepared statement ID.
//
// Layout based on the CQL native protocol v4 spec:
// /* <id: short bytes><consistency: short><flags: byte>... */
func PatchExecuteConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	if len(body) < 2 {
		return fmt.Errorf("body too short for EXECUTE")
	}
	idLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + idLen
	if len(body) < offset+2 {
		return fmt.Errorf("not enough bytes to patch EXECUTE consistency")
	}
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(newConsistency))
	return nil
}

func PatchBatchConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	//TODO: Implement this
	return nil
}
