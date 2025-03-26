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
//go:generate go fmt lexer.go

package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/google/uuid"
)

type Selector interface {
	Values(columns []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error)
	Columns(columns []*message.ColumnMetadata, stmt *SelectStatement) (filtered []*message.ColumnMetadata, err error)
}

type AliasSelector struct {
	Selector Selector
	Alias    string
}

func (a AliasSelector) Values(columns []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	return a.Selector.Values(columns, valueFunc)
}

func (a AliasSelector) Columns(columns []*message.ColumnMetadata, stmt *SelectStatement) (filtered []*message.ColumnMetadata, err error) {
	cols, err := a.Selector.Columns(columns, stmt)
	if err != nil {
		return
	}
	for _, column := range cols {
		alias := *column // Make a copy so we can modify the name
		alias.Name = a.Alias
		filtered = append(filtered, &alias)
	}
	return
}

type IDSelector struct {
	Name string
}

func (i IDSelector) Values(_ []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	value, err := valueFunc(i.Name)
	if err != nil {
		return
	}
	return []message.Column{value}, err
}

func (i IDSelector) Columns(columns []*message.ColumnMetadata, stmt *SelectStatement) (filtered []*message.ColumnMetadata, err error) {
	if column := FindColumnMetadata(columns, i.Name); column != nil {
		return []*message.ColumnMetadata{column}, nil
	} else {
		return nil, fmt.Errorf("invalid column %s", i.Name)
	}
}

type StarSelector struct{}

func (s StarSelector) Values(columns []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	for _, column := range columns {
		var val message.Column
		val, err = valueFunc(column.Name)
		if err != nil {
			return
		}
		filtered = append(filtered, val)
	}
	return
}

func (s StarSelector) Columns(columns []*message.ColumnMetadata, _ *SelectStatement) (filtered []*message.ColumnMetadata, err error) {
	filtered = columns
	return
}

type CountFuncSelector struct {
	Arg string
}

func (s CountFuncSelector) Values(_ []*message.ColumnMetadata, valueFunc ValueLookupFunc) (filtered []message.Column, err error) {
	val, err := valueFunc(CountValueName)
	if err != nil {
		return
	}
	filtered = append(filtered, val)
	return
}

func (s CountFuncSelector) Columns(_ []*message.ColumnMetadata, stmt *SelectStatement) (filtered []*message.ColumnMetadata, err error) {
	name := "count"
	if s.Arg != "*" {
		name = fmt.Sprintf("system.count(%s)", strings.ToLower(s.Arg))
	}
	return []*message.ColumnMetadata{{
		Keyspace: stmt.Keyspace,
		Table:    stmt.Table,
		Name:     name,
		Type:     datatype.Int,
	}}, nil
}

type NowFuncSelector struct{}

func (s NowFuncSelector) Values(_ []*message.ColumnMetadata, _ ValueLookupFunc) (filtered []message.Column, err error) {
	u, err := uuid.NewUUID()
	if err != nil {
		return
	}
	filtered = append(filtered, u[:])
	return
}

func (s NowFuncSelector) Columns(_ []*message.ColumnMetadata, stmt *SelectStatement) (filtered []*message.ColumnMetadata, err error) {
	return []*message.ColumnMetadata{{
		Keyspace: stmt.Keyspace,
		Table:    stmt.Table,
		Name:     "system.now()",
		Type:     datatype.Timeuuid,
	}}, nil
}
type Statement interface {
	isStatement()
}

type SelectStatement struct {
	Keyspace  string
	Table     string
	Selectors []Selector
}

func (s SelectStatement) isStatement() {}

type UseStatement struct {
	Keyspace string
}

func (u UseStatement) isStatement() {}

// IsQueryHandled parses the query string and determines if the query is handled by the proxy
func IsQueryHandled(keyspace Identifier, query string) (handled bool, stmt Statement, err error) {
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

// IsQueryIdempotent parses the query string and determines if the query is idempotent
func IsQueryIdempotent(query string) (idempotent bool, err error) {
	var l lexer
	l.init(query)
	return isIdempotentStmt(&l, l.next())
}

func isIdempotentStmt(l *lexer, t token) (idempotent bool, err error) {
	switch t {
	case tkSelect:
		return true, nil
	case tkUse, tkCreate, tkAlter, tkDrop:
		return false, nil
	case tkInsert:
		idempotent, t, err = isIdempotentInsertStmt(l)
	case tkUpdate:
		idempotent, t, err = isIdempotentUpdateStmt(l)
	case tkDelete:
		idempotent, t, err = isIdempotentDeleteStmt(l)
	case tkBegin:
		return isIdempotentBatchStmt(l)
	default:
		return false, errors.New("invalid statement type")
	}
	return idempotent && (t == tkEOF || t == tkEOS), err
}
