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

// Determine if where clause is idempotent for an UPDATE or DELETE mutation.
//
// whereClause: 'WHERE' relation ( 'AND' relation )*
//
func parseWhereClause(l *lexer) (idempotent bool, t token, err error) {
	for t = l.next(); tkIf != t && !isDMLTerminator(t); t = skipToken(l, l.next(), tkAnd) {
		idempotent, err = parseRelation(l, t)
		if !idempotent {
			return idempotent, tkInvalid, err
		}
	}
	return true, t, nil
}

// Determine if a relation is idempotent for an UPDATE or DELETE mutation.
//
// relation
// : identifier operator term
// | 'TOKEN' '(' identifiers ')' operator term
// | identifier 'LIKE' term
// | identifier 'IS' 'NOT' 'NULL'
// | identifier 'CONTAINS' 'KEY'? term
// | identifier '[' term ']' operator term
// | identifier 'IN' ( '(' terms? ')' | bindMarker )
// | '(' identifiers ')' 'IN' ( '(' terms? ')' | bindMarker )
// | '(' identifiers ')' operator ( '(' terms? ')' | bindMarker )
// | '(' relation ')'
//
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
				if tkRparen != t {
					return false, errors.New("expected closing ')' after terms")
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

// Determines if identifiers relation is idempotent.
//
//  ... 'IN' ( '(' terms? ')' | bindMarker )
//  ... operator ( '(' terms? ')' | bindMarker )
//
func parseIdentifiersRelation(l *lexer) (idempotent bool, err error) {
	switch t := l.next(); t {
	case tkIn, tkEqual, tkLt, tkLtEqual, tkGt, tkGtEqual, tkNotEqual:
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
		default:
			return false, errors.New("unexpected term token in identifiers relation")
		}
	default:
		return false, errors.New("unexpected token in identifiers relation")
	}

	return true, nil
}

// Parses the remainder of a bind marker.
//
// bindMarker
// : ':' identifier
// | '?'
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

func isOperator(t token) bool {
	return tkEqual == t || tkLt == t || tkLtEqual == t || tkGt == t || tkGtEqual == t || tkNotEqual == t
}
