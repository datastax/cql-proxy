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

// Determines if a delete statement is idempotent.
//
// A delete statement not idempotent if:
// * removes an element from a list
// * uses a lightweight transaction (LWT) e.g. 'IF EXISTS' or 'IF a > 0'
// * has a relation that uses a non-idempotent function e.g. now() or uuid()
//
// deleteStatement: 'DELETE'  deleteOperations? 'FROM' tableName ( 'USING' timestamp )? whereClause ( 'IF' ( 'EXISTS' | conditions ) )?
// deleteOperations: deleteOperation ( ',' deleteOperation )*
// deleteOperation: identifier | identifier '[' term ']'| identifier '.' identifier
// tableName: ( identifier '.' )? identifier
//
func isIdempotentDeleteStmt(l *lexer) (idempotent bool, err error) {
	t := l.next()
	for ; tkFrom != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
		if tkIdentifier != t {
			return false, errors.New("unexpected token after 'DELETE' in delete statement")
		}

		l.mark()
		switch t = l.next(); t {
		case tkLsquare:
			var typ termType
			if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
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

// Delete element terms can be one of the following:
// * Literal (idempotent, if not an integer literal)
// * Bind marker (ambiguous, so not idempotent)
// * Function call (ambiguous, so not idempotent)
// * Type cast (ambiguous)
func isIdempotentDeleteElementTermType(typ termType) bool {
	return typ != termIntegerLiteral && typ != termBindMarker && typ != termFunctionCall && typ != termCast
}
