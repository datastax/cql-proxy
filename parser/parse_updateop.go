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

// Determines if an update operation is idempotent.
//
// Non-idempotent update operations include:
// * Using a non-idempotent function e.g. now(), uuid()
// * Prepends or appends to a list type
// * Increments or decrements a counter
//
// Important: There are currently some ambiguous cases where if the type is not known we cannot correctly
// determine if an operation is idempotent. These include:
// * Using a bind marker (this could be fixed for prepared statements using the prepared metadata)
// * Function calls
//
// updateOperation
//    : identifier '=' term ( '+' identifier )?
//    | identifier '=' identifier ( '+' | '-' ) term
//    | identifier ( '+=' | '-=' ) term
//    | identifier '[' term ']' '=' term
//    | identifier '.' identifier '=' term
//
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
		if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
			return idempotent, err
		}
		if tkRsquare != l.next() {
			return false, errors.New("expected closing ']' in update operation")
		}
		if tkEqual != l.next() {
			return false, errors.New("expected '=' in update operation")
		}
		if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
			return idempotent, err
		}
	case tkDot: // identifier '.' identifier '=' term
		if tkIdentifier != l.next() {
			return false, errors.New("expected identifier after '.' in update operation")
		}
		if tkEqual != l.next() {
			return false, errors.New("expected '=' in update operation")
		}
		if idempotent, _, err = parseTerm(l, l.next()); !idempotent {
			return idempotent, err
		}
	default:
		return false, errors.New("unexpected token in update operation")
	}

	return true, nil
}

// Update terms can be one of the following:
// * Literal (idempotent, if not a list)
// * Bind marker (ambiguous, so not idempotent)
// * Function call (ambiguous, so not idempotent)
// * Type cast (probably not idempotent)
func isIdempotentUpdateOpTermType(typ termType) bool {
	return typ == termSetMapUdtLiteral || typ == termTupleLiteral
}
