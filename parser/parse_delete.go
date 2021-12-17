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
