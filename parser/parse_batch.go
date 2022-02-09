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

//  Determines if a batch statement is idempotent.
//
// A batch statement is not idempotent if:
// * it updates counters
// * contains DML statements that are not idempotent
//
// batchStatement: 'BEGIN' ( 'UNLOGGED' | 'COUNTER' )? 'BATCH'
//      usingClause?
//      ( batchChildStatement ';'? )*
//      'APPLY' 'BATCH'
//
// batchChildStatement: insertStatement | updateStatement | deleteStatement
//
func isIdempotentBatchStmt(l *lexer) (idempotent bool, err error) {
	t := l.next()

	if isUnreservedKeyword(l, t, "unlogged") {
		t = l.next()
	} else if isUnreservedKeyword(l, t, "counter") {
		return false, nil // Updates to counters are not idempotent
	}

	if tkBatch != t {
		return false, errors.New("expected 'BATCH' at the beginning of a batch statement")
	}

	t, err = parseUsingClause(l, l.next())
	if err != nil {
		return false, err
	}

	for tkApply != t && tkEOF != t {
		switch t {
		case tkInsert:
			idempotent, t, err = isIdempotentInsertStmt(l)
		case tkUpdate:
			idempotent, t, err = isIdempotentUpdateStmt(l)
		case tkDelete:
			idempotent, t, err = isIdempotentDeleteStmt(l)
		default:
			return false, errors.New("unexpected child statement in batch statement")
		}
		if t == tkEOS { // Skip ';'
			t = l.next()
		}
		if !idempotent {
			return idempotent, err
		}
	}

	if tkApply != t {
		return false, errors.New("expected 'APPLY' after child statements at the end of a batch statement")
	}

	if tkBatch != l.next() {
		return false, errors.New("expected 'BATCH' at the end of a batch statement")
	}

	return true, nil
}
