package parser

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParser(t *testing.T) {
	var tests = []struct {
		query  string
		stmt   StmtType
		target string
	}{
		{"SELECT * FROM system.local key='local';", StmtSelect, "system.local"},
		{"SELECT * FROM SysteM.LocaL", StmtSelect, "system.local"},
		{`SELECT * FROM "system"."local"`, StmtSelect, "system.local"},
		{`SELECT * FROM "SysteM"."LocaL"`, StmtSelect, "SysteM.LocaL"},
		{"SELECT system.local", StmtUnknown, ""},
		{`SELECT * FROM "system.local`, StmtUnknown, ""},
		{`; SELECT * FROM "system.local`, StmtUnknown, ""},
		{"USE system;", StmtUse, "system"},
		{`USE "system";`, StmtUse, "system"},
		{`USE "SysteM";`, StmtUse, "SysteM"},
		{`USE "system`, StmtUnknown, ""},
		{`USE ; system`, StmtUnknown, ""},
		{`SELECT USE`, StmtUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			stmt, target := Parse(tt.query)
			assert.Equal(t, tt.stmt, stmt)
			assert.Equal(t, tt.target, target)
		})
	}
}
