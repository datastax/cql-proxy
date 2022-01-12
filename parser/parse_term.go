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

// Determines if a term is idempotent and also returns the top-level type of the term.
//
// A term is not idempotent if it contains a non-idempotent function e.g. 'now()' or 'uuid()'
//
// term: literal | bindMarker | functionCall | typeCast
//
// literal: primitiveLiteral | collectionLiteral | tupleLiteral | udtLiteral | 'NULL'
// primitiveLiteral:  stringLiteral | integer | float | boolean | duration | uuid | hexNumber | '-'? 'NAN' | '-'? 'INFINITY'
// collectionLiteral: listLiteral | setLiteral | mapLiteral
// listLiteral: '[' terms? ']'
// setLiteral: '{' terms? '}'
// mapLiteral: '{' ( mapEntry ( ',' mapEntry )* )? '}'
// mapEntry: term ':' term
// tupleLiteral: '(' terms? ')'
// udtLiteral: '{' ( fieldEntry ( ',' fieldEntry )* )? '}'
// fieldEntry: identifier ':' term
//
// bindMarker: '?' | ':' identifier
//
// functionCall: ( identifier '.' )? identifier '(' functionArg ( ',' functionArg )* ')'
// functionArg: identifier | term
//
// typeCast: '(' type ')' term
// type: identifier | identifier '<' type '>'
//
func parseTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	switch t {
	case tkInteger: // Integer lister
		return true, termIntegerLiteral, nil
	case tkFloat, tkBool, tkNull, tkStringLiteral, tkHexNumber, tkUuid, tkDuration, tkNan, tkInfinity: // Literal
		return true, termPrimitiveLiteral, nil
	case tkColon: // Named bind marker
		if t = l.next(); t != tkIdentifier {
			return false, termBindMarker, errors.New("expected identifier after ':' for named bind marker")
		}
		return true, termBindMarker, nil
	case tkQMark: // Positional bind marker
		return true, termBindMarker, nil
	case tkLsquare: // List literal
		return parseListTerm(l)
	case tkLcurly: // Set, map, or UDT literal
		if t = l.next(); t == tkIdentifier { // maybe UDT
			l.mark()
			var maybeColon token
			_, _, maybeColon, err = parseQualifiedIdentifier(l)
			if err != nil {
				return false, termSetMapUdtLiteral, err
			}
			l.rewind()
			if tkColon == maybeColon { // UDT
				return parseUDTTerm(l, t)
			} else { // Set or map (probably starting with a function)
				return parseSetOrMapTerm(l, t)
			}
		} else { // Set or map
			return parseSetOrMapTerm(l, t)
		}
	case tkLparen: // Type cast or tuple literal
		if t = l.next(); t == tkIdentifier { // Cast
			return parseCastTerm(l, t)
		} else { // Tuple literal
			return parseTupleTerm(l, t)
		}
	case tkIdentifier: // Function
		return parseFunctionTerm(l)
	}

	return false, termInvalid, errors.New("invalid term")
}

func parseListTerm(l *lexer) (idempotent bool, typ termType, err error) {
	var t token
	for t = l.next(); t != tkRsquare && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, termListLiteral, err
		}
	}
	if t != tkRsquare {
		return false, termListLiteral, errors.New("expected closing ']' bracket for list literal")
	}
	return true, termListLiteral, nil
}

func parseUDTTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	for t != tkRcurly && t != tkEOF {
		if tkIdentifier != t {
			return false, termSetMapUdtLiteral, errors.New("expected identifier in UDT literal field")
		}
		_, _, t, err = parseQualifiedIdentifier(l)
		if err != nil {
			return false, termSetMapUdtLiteral, err
		}
		t = skipToken(l, l.next(), tkColon)
		if idempotent, typ, err = parseTerm(l, t); !idempotent {
			return idempotent, termSetMapUdtLiteral, err
		}
		t = skipToken(l, l.next(), tkComma)
	}
	if t != tkRcurly {
		return false, termSetMapUdtLiteral, errors.New("expected closing '}' bracket for UDT literal")
	}
	return true, termSetMapUdtLiteral, nil
}

func parseSetOrMapTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	for t != tkRcurly && t != tkEOF {
		if idempotent, typ, err = parseTerm(l, t); !idempotent {
			return idempotent, termSetMapUdtLiteral, err
		}
		if t = l.next(); tkColon == t { // Map
			if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
				return idempotent, termSetMapUdtLiteral, err
			}
			t = l.next()
		}
		t = skipToken(l, t, tkComma)
	}
	if t != tkRcurly {
		return false, termSetMapUdtLiteral, errors.New("expected closing '}' bracket for set/map literal")
	}
	return true, termSetMapUdtLiteral, nil
}

func parseCastTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	t, err = parseType(l)
	if err != nil {
		return false, termCast, err
	}
	if t != tkRparen {
		return false, termCast, errors.New("expected closing ')' bracket for type cast")
	}
	if idempotent, typ, err = parseTerm(l, l.next()); !idempotent {
		return idempotent, termCast, err
	}
	return true, termCast, err
}

func parseTupleTerm(l *lexer, t token) (idempotent bool, typ termType, err error) {
	for t != tkRparen && t != tkEOF {
		if idempotent, _, err = parseTerm(l, t); !idempotent {
			return idempotent, termTupleLiteral, err
		}
		t = skipToken(l, l.next(), tkComma)
	}
	if t != tkRparen {
		return false, termTupleLiteral, errors.New("expected closing ')' bracket for tuple literal")
	}
	return true, termTupleLiteral, nil
}

func parseFunctionTerm(l *lexer) (idempotent bool, typ termType, err error) {
	var target, keyspace Identifier
	keyspace, target, t, err := parseQualifiedIdentifier(l)
	if err != nil {
		return false, termFunctionCall, err
	}
	if tkLparen != t {
		return false, termFunctionCall, errors.New("invalid term, was expecting function call")
	}
	for t = l.next(); t != tkRparen && t != tkEOF; t = skipToken(l, l.next(), tkComma) {
		l.mark()
		maybeCommaOrRparen := l.next()
		if tkIdentifier == t && (tkComma == maybeCommaOrRparen || tkRparen == maybeCommaOrRparen) {
			l.rewind()
		} else {
			l.rewind()
			if idempotent, _, err = parseTerm(l, t); !idempotent {
				return idempotent, termFunctionCall, err
			}
		}
	}
	if t != tkRparen {
		return false, termFunctionCall, errors.New("expected closing ')' for function call")
	}
	return !(isNonIdempotentFunc(target) && (keyspace.isEmpty() || keyspace.equal("system"))), termFunctionCall, nil
}

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
