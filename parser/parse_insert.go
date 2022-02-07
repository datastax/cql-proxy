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

import "errors"

// Determines if an insert statement is idempotent.
//
// An insert statement is not idempotent if it contains a non-idempotent term e.g. 'now()' or 'uuid()' or if it uses
// lightweight transactions (LWTs) e.g. using 'IF NOT EXISTS'.
//
// insertStatement: 'INSERT' 'INTO' tableName ( namedValues | jsonClause ) ( 'IF' 'NOT' 'EXISTS' )?
// tableName: ( identifier '.' )? identifier
// namedValues: '(' identifiers ')' 'VALUES' '(' terms ')'
// jsonClause: 'JSON' stringLiteral ( 'DEFAULT' ( 'NULL' | 'UNSET' ) )?
//
func isIdempotentInsertStmt(l *lexer) (idempotent bool, t token, err error) {
	t = l.next()
	if tkInto != t {
		return false, tkInvalid, errors.New("expected 'INTO' after 'INSERT' for insert statement")
	}

	if t = l.next(); tkIdentifier != t {
		return false, tkInvalid, errors.New("expected identifier after 'INTO' in insert statement")
	}

	_, _, t, err = parseQualifiedIdentifier(l)
	if err != nil {
		return false, tkInvalid, err
	}

	if !isUnreservedKeyword(l, t, "json") {
		if tkLparen != t {
			return false, tkInvalid, errors.New("expected '(' after table name for insert statement")
		}

		err = parseIdentifiers(l, l.next())
		if err != nil {
			return false, tkInvalid, err
		}

		if !isUnreservedKeyword(l, l.next(), "values") {
			return false, tkInvalid, errors.New("expected 'VALUES' after identifiers in insert statement")
		}

		if t != l.next() {
			return false, tkInvalid, errors.New("expected '(' after 'VALUES' in insert statement")
		}

		for t = l.next(); tkRparen != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, tkInvalid, err
			}
		}

		if t != tkRparen {
			return false, tkInvalid, errors.New("expected closing ')' for 'VALUES' list in insert statement")
		}
	}

	for t = l.next(); !isDMLTerminator(t); t = l.next() {
		if tkIf == t {
			return false, tkInvalid, nil
		}
	}
	return true, t, nil
}
