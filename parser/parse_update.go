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

// Determines if an update statement is idempotent.
//
// updateStatement: 'UPDATE' tableName usingClause? 'SET' updateOperations whereClause( 'IF' ( 'EXISTS' | conditions ))?
// tableName: ( identifier '.' )? identifier
//
func isIdempotentUpdateStmt(l *lexer) (idempotent bool, err error) {
	t := l.next()
	if tkIdentifier != t {
		return false, errors.New("expected identifier after 'UPDATE' in update statement")
	}

	_, _, t, err = parseQualifiedIdentifier(l)
	if err != nil {
		return false, err
	}

	t, err = parseUsingClause(l, t)
	if err != nil {
		return false, err
	}

	for !isUnreservedKeyword(l, t, "set") {
		return false, errors.New("expected 'SET' in update statement")
	}

	for t = l.next(); tkIf != t && tkWhere != t && tkEOF != t; t = skipToken(l, l.next(), tkComma) {
		idempotent, err = parseUpdateOp(l, t)
		if !idempotent {
			return idempotent, err
		}
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

// Parse over using clause.
//
// usingClause
//	  : 'USING' timestamp
//    | 'USING' ttl
//    | 'USING' timestamp 'AND' ttl
//    | 'USING' ttl 'AND' timestamp
//
func parseUsingClause(l *lexer, t token) (next token, err error) {
	if tkUsing == t {
		err = parseTtlOrTimestamp(l)
		if err != nil {
			return tkInvalid, err
		}
		if t = l.next(); tkAnd == t {
			err = parseTtlOrTimestamp(l)
			if err != nil {
				return tkInvalid, err
			}
		}
	}
	return t, nil
}

// Parse over TTL or timestamp
//
// timestamp: 'TIMESTAMP' ( INTEGER | bindMarker )
// ttl: 'TTL' ( INTEGER | bindMarker )
//
func parseTtlOrTimestamp(l *lexer) error {
	var t token
	if t = l.next(); !isUnreservedKeyword(l, t, "ttl") && !isUnreservedKeyword(l, t, "timestamp") {
		return errors.New("expected 'TTL' or 'TIMESTAMP' after 'USING'")
	}
	switch t = l.next(); t {
	case tkInteger:
		return nil
	case tkColon, tkQMark:
		return parseBindMarker(l, t)
	}
	return errors.New("expected integer or bind marker after 'TTL' or 'TIMESTAMP'")
}
