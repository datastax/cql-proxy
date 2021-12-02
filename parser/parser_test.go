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
		stmt       interface{}
	}{
		{"", "SELECT key, rpc_address AS address, count(*) FROM system.local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []interface{}{
				&IDSelector{Name: "key"},
				&AliasSelector{Alias: "address", Selector: &IDSelector{Name: "rpc_address"}},
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"system", "SELECT count(*) FROM local", true, true, &SelectStatement{
			Table: "local",
			Selectors: []interface{}{
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"", "SELECT count(*) FROM system.peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []interface{}{
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"system", "SELECT count(*) FROM peers", true, true, &SelectStatement{
			Table: "peers",
			Selectors: []interface{}{
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"", "SELECT count(*) FROM system.peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []interface{}{
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"system", "SELECT count(*) FROM peers_v2", true, true, &SelectStatement{
			Table: "peers_v2",
			Selectors: []interface{}{
				&CountStarSelector{Name: "count(*)"},
			},
		}},
		{"", "SELECT func(key) FROM system.local", true, true, &ErrorSelectStatement{
			Err: errors.New("unsupported select clause for system table"),
		}},
		{"", "USE system", true, false, &UseStatement{
			Keyspace: "system",
		}},
		{"", "SELECT count(*) FROM local", false, true, nil},
		{"", "SELECT count(*) FROM peers", false, true, nil},
		{"", "SELECT count(*) FROM peers_v2", false, true, nil},
		{"", "INSERT INTO system.local (key, rpc_address) VALUES ('local1', '127.0.0.1')", false, false, nil},
		{"", "UPDATE system.local SET rpc_address = '127.0.0.1' WHERE key = 'local'", false, false, nil},
		{"", "DELETE rpc_address FROM system.local WHERE key = 'local'", false, false, nil},
	}

	for _, tt := range tests {
		handled, idempotent, stmt := Parse(tt.keyspace, tt.query)
		assert.Equal(t, tt.handled, handled, "invalid handled")
		assert.Equal(t, tt.idempotent, idempotent, "invalid idempotency")
		assert.Equal(t, tt.stmt, stmt, "invalid parsed statement")
	}
}

func TestParser_Idempotent(t *testing.T) {
	_, idempotent, _ := Parse("", "UPDATE table SET v = v + [1] WHERE k = 1")
	assert.False(t, idempotent)
}
