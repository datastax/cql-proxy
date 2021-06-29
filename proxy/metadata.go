package proxy

import (
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

var (
	systemLocalColumns = []*message.ColumnMetadata{
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

	systemPeersColumns = []*message.ColumnMetadata{
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

func findColumnMetadata(columns []*message.ColumnMetadata, name string) *message.ColumnMetadata {
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	return nil
}
