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

import "errors"

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
	case tkUse:
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
	return idempotent && t == tkEOF, err
}
