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
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

var (
	SystemLocalColumns = []*message.ColumnMetadata{
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "key",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "rpc_address",
			Type:     datatype.Inet,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "data_center",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "rack",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "tokens",
			Type:     datatype.NewSetType(datatype.Varchar),
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "release_version",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "partitioner",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "cluster_name",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "cql_version",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "schema_version",
			Type:     datatype.Uuid,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "native_protocol_version",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "local",
			Name:     "host_id",
			Type:     datatype.Uuid,
		},
	}

	SystemPeersColumns = []*message.ColumnMetadata{
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "peer",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "rpc_address",
			Type:     datatype.Inet,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "data_center",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "rack",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "tokens",
			Type:     datatype.NewSetType(datatype.Varchar),
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "release_version",
			Type:     datatype.Varchar,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "schema_version",
			Type:     datatype.Uuid,
		},
		{
			Keyspace: "system",
			Table:    "peers",
			Name:     "host_id",
			Type:     datatype.Uuid,
		},
	}
)

func FindColumnMetadata(columns []*message.ColumnMetadata, name string) *message.ColumnMetadata {
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	return nil
}
