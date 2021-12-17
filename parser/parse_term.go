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
)

type termType int

const (
	termInvalid        termType = iota
	termIntegerLiteral          // Special because it can be used to index lists for deletes
	termPrimitiveLiteral
	termListLiteral
	termSetMapUdtLiteral // All use curly, distinction not important
	termTupleLiteral
	termBindMarker
	termFunctionCall
	termCast
)

func parseType(l *lexer) (t token, err error) {
	if t = l.next(); tkLangle == t {
		for t = l.next(); tkRangle != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
			if t != tkIdentifier {
				return tkInvalid, errors.New("expected sub-type in type parameter")
			}
		}
		if tkRangle != t {
			return tkInvalid, errors.New("expected closing '>' bracket for type")
		}
		return l.next(), nil
	}
	return t, nil
}

func parseTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	switch t {
	case tkInteger:
		return true, termIntegerLiteral, nil
	case tkFloat, tkBool, tkNull, tkStringLiteral, tkHexNumber, tkUuid, tkDuration, tkNan, tkInfinity: // Literal
		return true, termPrimitiveLiteral, nil
	case tkLsquare: // List literal
		for t = l.next(); t != tkRsquare && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, termInvalid, err
			}
		}
		if t != tkRsquare {
			return false, termInvalid, errors.New("expected closing ']' bracket for list literal")
		}
		return true, termListLiteral, nil
	case tkLcurly: // Set, map, or UDT literal
		if t = l.next(); t == tkIdentifier { // UDT
			for t != tkRcurly && t != tkEOF {
				_, _, t, err = parseQualifiedIdentifier(l)
				if err != nil {
					return false, termInvalid, err
				}
				t = skipToken(l, l.next(), tkColon)
				if idempotent, typ, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				t = skipToken(l, l.next(), tkComma)
			}
		} else {
			for t != tkRcurly && t != tkEOF {
				if idempotent, typ, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				if t = l.next(); tkColon == t { // Map
					if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
						return idempotent, termInvalid, err
					}
					t = l.next()
				}
				t = skipToken(l, t, tkComma)
			}
		}
		if t != tkRcurly {
			return false, termInvalid, errors.New("expected closing '}' bracket for set/map/UDT literal")
		}
		return true, termSetMapUdtLiteral, nil
	case tkLparen: // Type cast or tuple literal
		if t = l.next(); t == tkIdentifier { // Cast
			t, err = parseType(l)
			if err != nil {
				return false, termInvalid, err
			}
			if t != tkRparen {
				return false, termInvalid, errors.New("expected closing ')' bracket for type cast")
			}
			if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, termInvalid, err
			}
			return true, termCast, err
		} else { // Tuple literal
			for t != tkRparen && t != tkEOF {
				if idempotent, _, err = parseTerm(l, t); !idempotent {
					return idempotent, termInvalid, err
				}
				t = skipToken(l, l.next(), tkComma)
			}
			if t != tkRparen {
				return false, termInvalid, errors.New("expected closing ')' bracket for tuple literal")
			}
			return true, termTupleLiteral, nil
		}
	case tkColon: // Named bind marker
		if t = l.next(); t != tkIdentifier {
			return false, termInvalid, errors.New("expected identifier after ':' for named bind marker")
		}
		return true, termBindMarker, nil
	case tkQMark: // Positional bind marker
		return true, termBindMarker, nil
	case tkIdentifier: // Function
		var target, keyspace Identifier
		keyspace, target, t, err = parseQualifiedIdentifier(l)
		if err != nil {
			return false, termInvalid, err
		}
		if tkLparen != t {
			return false, termInvalid, errors.New("invalid term, was expecting function call")
		}
		for t = l.next(); t != tkRparen && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, termInvalid, err
			}
		}
		if t != tkRparen {
			return false, termInvalid, errors.New("expected closing ')' for function call")
		}
		return !(isNonIdempotentFunc(target) && (keyspace.isEmpty() || keyspace.equal("system"))), termFunctionCall, nil
	}

	return false, termInvalid, errors.New("invalid term")
}
