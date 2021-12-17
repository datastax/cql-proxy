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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	var tests = []struct {
		keyspace   string
		query      string
		handled    bool
		idempotent bool
		stmt       Statement
		err        error
	}{
		{"", "SELECT key, rpc_address AS address, count(*) FROM system.local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []Selector{
				&IDSelector{Name: "key"},
				&AliasSelector{Alias: "address", Selector: &IDSelector{Name: "rpc_address"}},
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM \"local\"", true, true, &SelectStatement{
			Table: "local",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT count(*) FROM system.peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT count(*) FROM \"system\".\"peers\"", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT count(*) FROM system.peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"system", "SELECT count(*) FROM peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []Selector{
				&CountStarSelector{Name: "count(*)"},
			},
		}, nil},
		{"", "SELECT func(key) FROM system.local", true, true, nil,
			errors.New("unsupported select clause for system table")},
		{"", "USE system", true, false, &UseStatement{
			Keyspace: "system",
		}, nil},
		// Reads from tables named similarly to system tables (not handled)
		{"", "SELECT count(*) FROM local", false, true, nil, nil},
		{"", "SELECT count(*) FROM peers", false, true, nil, nil},
		{"", "SELECT count(*) FROM peers_v2", false, true, nil, nil},

		// Mutations to system tables (not handled)
		{"", "INSERT INTO system.local (key, rpc_address) VALUES ('local1', '127.0.0.1')", false, false, nil, nil},
		{"", "UPDATE system.local SET rpc_address = '127.0.0.1' WHERE key = 'local'", false, false, nil, nil},
		{"", "DELETE rpc_address FROM system.local WHERE key = 'local'", false, false, nil, nil},
	}

	for _, tt := range tests {
		handled, stmt, err := IsQueryHandled(IdentifierFromString(tt.keyspace), tt.query)
		assert.Equal(t, tt.err, err, tt.query)

		idempotent, err := IsQueryIdempotent(tt.query)
		assert.Nil(t, err, tt.query)

		assert.Equal(t, tt.handled, handled, "invalid handled", tt.query)
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency", tt.query)
		assert.Equal(t, tt.stmt, stmt, "invalid parsed statement", tt.query)
	}
}
