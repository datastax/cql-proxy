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

package proxycore

import (
	"errors"
	"fmt"
	"net"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var (
	ColumnNameNotFound = errors.New("column name not found")
	ColumnIsNull       = errors.New("column is null")
)

type ResultSet struct {
	columnIndexes map[string]int
	result        *message.RowsResult
	version       primitive.ProtocolVersion
}

type Row struct {
	resultSet *ResultSet
	row       message.Row
}

func NewResultSet(rows *message.RowsResult, version primitive.ProtocolVersion) *ResultSet {
	columnIndexes := make(map[string]int)
	for i, column := range rows.Metadata.Columns {
		columnIndexes[column.Name] = i
	}
	return &ResultSet{
		columnIndexes: columnIndexes,
		result:        rows,
		version:       version,
	}
}

func (rs *ResultSet) Row(i int) Row {
	return Row{
		rs,
		rs.result.Data[i]}
}

func (rs ResultSet) RowCount() int {
	return len(rs.result.Data)
}

func (r Row) ByPos(i int) (interface{}, error) {
	val, err := DecodeType(r.resultSet.result.Metadata.Columns[i].Type, r.resultSet.version, r.row[i])
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (r Row) ByName(n string) (interface{}, error) {
	if i, ok := r.resultSet.columnIndexes[n]; !ok {
		return nil, ColumnNameNotFound
	} else {
		return r.ByPos(i)
	}
}

func (r Row) StringByName(n string) (string, error) {
	val, err := r.ByName(n)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", ColumnIsNull
	} else if s, ok := val.(string); !ok {
		return "", fmt.Errorf("'%s' is not a string", n)
	} else {
		return s, nil
	}
}

func (r Row) InetByName(n string) (net.IP, error) {
	val, err := r.ByName(n)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, ColumnIsNull
	} else if ip, ok := val.(net.IP); !ok {
		return nil, fmt.Errorf("'%s' is not an inet (or is null)", n)
	} else {
		return ip, nil
	}
}

func (r Row) UUIDByName(n string) (primitive.UUID, error) {
	val, err := r.ByName(n)
	if err != nil {
		return [16]byte{}, err
	}
	if val == nil {
		return [16]byte{}, ColumnIsNull
	} else if u, ok := val.(primitive.UUID); !ok {
		return [16]byte{}, fmt.Errorf("'%s' is not a uuid (or is null)", n)
	} else {
		return u, nil
	}
}
