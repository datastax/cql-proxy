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

import "strings"

// Identifier is a CQL identifier
type Identifier struct {
	id         string
	ignoreCase bool
}

// IdentifierFromString creates an identifier from a string
func IdentifierFromString(id string) Identifier {
	l := len(id)
	if l > 0 && id[0] == '"' {
		return Identifier{id: id[1 : l-1], ignoreCase: false}
	} else {
		return Identifier{id: id, ignoreCase: true}
	}
}

// correctly compares an identifier with a string
func (i Identifier) equal(id string) bool {
	if i.ignoreCase {
		return strings.EqualFold(i.id, id)
	} else {
		return i.id == id
	}
}

func (i Identifier) isEmpty() bool {
	return len(i.id) == 0
}
