package codecs

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartialQueryCodec_Decode(t *testing.T) {
	localSerialConsistency := primitive.ConsistencyLevelLocalSerial
	codec := &partialQueryCodec{}

	msg := &message.Query{
		Query: "SELECT * FROM table WHERE id = ?",
		Options: &message.QueryOptions{
			Consistency:       primitive.ConsistencyLevelOne,
			SerialConsistency: &localSerialConsistency,
		},
	}

	var buf bytes.Buffer
	err := builtinQueryCodec.Encode(msg, &buf, primitive.ProtocolVersion4)
	require.NoError(t, err)

	query, err := codec.Decode(NewFrameBodyReader(buf.Bytes()), primitive.ProtocolVersion4)
	require.NoError(t, err)

	assert.False(t, query.IsResponse())
	assert.Equal(t, primitive.OpCodeQuery, query.GetOpCode())
	assert.Equal(t, fmt.Sprintf("QUERY \"%s\"", msg.Query), query.(*PartialQuery).String())

	assert.Equal(t, msg.Query, query.(*PartialQuery).Query)
	assert.Equal(t, primitive.ConsistencyLevelOne, query.(*PartialQuery).Consistency)
	assert.Equal(t, []byte{0x10, 0x00, 0x9}, query.(*PartialQuery).Parameters)
}

func TestPartialQueryCodec_Decode_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		version primitive.ProtocolVersion
		buf     []byte
		err     string
	}{
		{
			name: "invalid query",
			buf:  []byte{},
			err:  "cannot read [long string] length: cannot read [int]: EOF",
		},
		{
			name: "invalid consistency level",
			buf:  []byte{0x00, 0x00, 0x00, 0x00},
			err:  "cannot read QUERY consistency level: cannot read [short]: EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &partialQueryCodec{}
			reader := NewFrameBodyReader(tt.buf)
			_, err := c.Decode(reader, tt.version)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestPartialQueryCodec_Encode(t *testing.T) {
	tests := []struct {
		name string
		msg  *message.Query
	}{
		{
			name: "simple",
			msg: &message.Query{
				Query: "select * from table1",
			},
		},
		{
			name: "consistency",
			msg: &message.Query{
				Query: "select * from table2",
				Options: &message.QueryOptions{
					Consistency: primitive.ConsistencyLevelEachQuorum,
				},
			},
		},
		{
			name: "parameters",
			msg: &message.Query{
				Query: "select * from table2",
				Options: &message.QueryOptions{
					Consistency: primitive.ConsistencyLevelEachQuorum,
					PositionalValues: []*primitive.Value{
						{
							Type:     primitive.ValueTypeRegular,
							Contents: []byte{0x1, 0x2, 0x3},
						},
						{
							Type:     primitive.ValueTypeRegular,
							Contents: []byte{0x4, 0x5, 0x6},
						},
					},
					SkipMetadata:    true,
					PageSize:        123,
					PageSizeInBytes: false,
					PagingState:     []byte{0xa, 0xb, 0xc},
					Keyspace:        "keyspace1",
				},
			},
		},
	}

	codec := &partialQueryCodec{}
	version := primitive.ProtocolVersion4

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen, err := builtinQueryCodec.EncodedLength(tt.msg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with built-in query codec")

			var expectedBytes bytes.Buffer
			err = builtinQueryCodec.Encode(tt.msg, &expectedBytes, version)
			require.NoError(t, err, "unexpected error encoding with built-in query codec")

			partialMsg, err := codec.Decode(NewFrameBodyReader(expectedBytes.Bytes()), version)
			assert.NoError(t, err, "unexpected error decoding partial query")

			// Sanity checks
			assert.Equal(t, tt.msg.Query, partialMsg.(*PartialQuery).Query)
			if tt.msg.Options != nil {
				assert.Equal(t, tt.msg.Options.Consistency, partialMsg.(*PartialQuery).Consistency)
			}

			actualLen, err := codec.EncodedLength(partialMsg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with partial query codec")

			assert.Equal(t, expectedLen, actualLen)

			var actualBytes bytes.Buffer
			err = codec.Encode(tt.msg, &actualBytes, version)
			require.NoError(t, err, "unexpected error encoding with partial query codec")

			assert.Equal(t, expectedBytes, actualBytes)
		})
	}
}

func TestPartialExecuteCodec_Decode(t *testing.T) {
	localSerialConsistency := primitive.ConsistencyLevelLocalSerial
	codec := &partialExecuteCodec{}

	msg := &message.Execute{
		QueryId:          []byte{0x0a, 0x0b, 0x0c},
		ResultMetadataId: []byte{0x0d, 0x0e, 0x0f},
		Options: &message.QueryOptions{
			Consistency:       primitive.ConsistencyLevelOne,
			SerialConsistency: &localSerialConsistency,
		},
	}

	var buf bytes.Buffer
	err := builtinExecuteCodec.Encode(msg, &buf, primitive.ProtocolVersion4)
	require.NoError(t, err)

	execute, err := codec.Decode(NewFrameBodyReader(buf.Bytes()), primitive.ProtocolVersion4)
	require.NoError(t, err)

	assert.False(t, execute.IsResponse())
	assert.Equal(t, primitive.OpCodeExecute, execute.GetOpCode())
	assert.Equal(t, "EXECUTE 0a0b0c", execute.(*PartialExecute).String())

	assert.Equal(t, []byte{0x0a, 0x0b, 0x0c}, execute.(*PartialExecute).QueryId)
	assert.Equal(t, primitive.ConsistencyLevelOne, execute.(*PartialExecute).Consistency)
	assert.Equal(t, []byte{0x10, 0x00, 0x9}, execute.(*PartialExecute).Parameters)
}

func TestPartialExecuteCodec_Decode_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		version primitive.ProtocolVersion
		buf     []byte
		err     string
	}{
		{
			name: "invalid query id",
			buf:  []byte{},
			err:  "cannot read EXECUTE query id: cannot read [short bytes] length: cannot read [short]: EOF",
		},
		{
			name: "empty query id",
			buf:  []byte{0x00, 0x00},
			err:  "EXECUTE missing query id",
		},
		{
			name: "invalid consistency level",
			buf:  []byte{0x00, 0x01, 0x00},
			err:  "cannot read EXECUTE consistency level: cannot read [short]: EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &partialExecuteCodec{}
			reader := NewFrameBodyReader(tt.buf)
			_, err := c.Decode(reader, tt.version)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestPartialExecuteCodec_Encode(t *testing.T) {
	tests := []struct {
		name string
		msg  *message.Execute
	}{
		{
			name: "simple",
			msg: &message.Execute{
				QueryId:          []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				ResultMetadataId: []byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0xa, 0xb},
			},
		},
		{
			name: "consistency",
			msg: &message.Execute{
				QueryId:          []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				ResultMetadataId: []byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0xa, 0xb},
				Options: &message.QueryOptions{
					Consistency: primitive.ConsistencyLevelEachQuorum,
				},
			},
		},
		{
			name: "parameters",
			msg: &message.Execute{
				QueryId:          []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				ResultMetadataId: []byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0xa, 0xb},
				Options: &message.QueryOptions{
					Consistency: primitive.ConsistencyLevelEachQuorum,
					PositionalValues: []*primitive.Value{
						{
							Type:     primitive.ValueTypeRegular,
							Contents: []byte{0x1, 0x2, 0x3},
						},
						{
							Type:     primitive.ValueTypeRegular,
							Contents: []byte{0x4, 0x5, 0x6},
						},
					},
					SkipMetadata:    true,
					PageSize:        123,
					PageSizeInBytes: false,
					PagingState:     []byte{0xa, 0xb, 0xc},
					Keyspace:        "keyspace1",
				},
			},
		},
	}

	codec := &partialExecuteCodec{}
	version := primitive.ProtocolVersion4

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen, err := builtinExecuteCodec.EncodedLength(tt.msg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with built-in execute codec")

			var expectedBytes bytes.Buffer
			err = builtinExecuteCodec.Encode(tt.msg, &expectedBytes, version)
			require.NoError(t, err, "unexpected error encoding with built-in execute codec")

			partialMsg, err := codec.Decode(NewFrameBodyReader(expectedBytes.Bytes()), version)
			assert.NoError(t, err, "unexpected error decoding partial execute")

			// Sanity checks
			assert.Equal(t, tt.msg.QueryId, partialMsg.(*PartialExecute).QueryId)
			if tt.msg.Options != nil {
				assert.Equal(t, tt.msg.Options.Consistency, partialMsg.(*PartialExecute).Consistency)
			}

			actualLen, err := codec.EncodedLength(partialMsg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with partial execute codec")

			assert.Equal(t, expectedLen, actualLen)

			var actualBytes bytes.Buffer
			err = codec.Encode(tt.msg, &actualBytes, version)
			require.NoError(t, err, "unexpected error encoding with partial execute codec")

			assert.Equal(t, expectedBytes, actualBytes)
		})
	}
}

func TestPartialBatchCodec_Decode(t *testing.T) {
	localSerialConsistency := primitive.ConsistencyLevelLocalSerial
	codec := &partialBatchCodec{}

	msg := &message.Batch{
		Type: primitive.BatchTypeLogged,
		Children: []*message.BatchChild{
			{
				Query: "SELECT * FROM table WHERE id = ?",
				Values: []*primitive.Value{
					{Type: primitive.ValueTypeRegular, Contents: []byte{0x01, 0x02, 0x03}},
					{Type: primitive.ValueTypeNull},
				},
			},
			{Id: []byte{0x01, 0x02, 0x03}},
		},
		Consistency:       primitive.ConsistencyLevelQuorum,
		SerialConsistency: &localSerialConsistency,
	}

	var buf bytes.Buffer

	err := builtinBatchCodec.Encode(msg, &buf, primitive.ProtocolVersion4)
	require.NoError(t, err)

	batch, err := codec.Decode(NewFrameBodyReader(buf.Bytes()), primitive.ProtocolVersion4)
	require.NoError(t, err)

	assert.False(t, batch.IsResponse())
	assert.Equal(t, primitive.OpCodeBatch, batch.GetOpCode())
	assert.Equal(t, fmt.Sprintf("BATCH (%d statements)", len(msg.Children)), batch.(*PartialBatch).String())

	require.Len(t, batch.(*PartialBatch).Queries, 2)
	assert.Equal(t, "SELECT * FROM table WHERE id = ?", batch.(*PartialBatch).Queries[0].QueryOrId)
	assert.Equal(t, []byte{0x0, 0x2, 0x0, 0x0, 0x0, 0x3, 0x1, 0x2, 0x3, 0xff, 0xff, 0xff, 0xff},
		batch.(*PartialBatch).Queries[0].Values)

	assert.Equal(t, []byte{0x1, 0x2, 0x3}, batch.(*PartialBatch).Queries[1].QueryOrId)
	assert.Equal(t, []byte{0x0, 0x0}, batch.(*PartialBatch).Queries[1].Values)

	assert.Equal(t, msg.Type, batch.(*PartialBatch).Type)
	assert.Equal(t, msg.Consistency, batch.(*PartialBatch).Consistency)
	assert.Equal(t, []byte{0x10, 0x0, 0x9}, batch.(*PartialBatch).Parameters)
}

func TestPartialBatchCodec_Decode_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		version primitive.ProtocolVersion
		buf     []byte
		err     string
	}{
		{
			name: "empty",
			buf:  []byte{},
			err:  "cannot read BATCH type: cannot read [byte]: EOF",
		},
		{
			name: "invalid batch type",
			buf:  []byte{0x0F}, // Invalid batch type
			err:  "invalid BATCH type: BatchType ? [0X0F]",
		},
		{
			name: "invalid batch query count",
			buf:  []byte{0x01},
			err:  "cannot read BATCH query count: cannot read [short]: EOF",
		},
		{
			name: "invalid batch query type",
			buf:  []byte{0x01, 0x00, 0x01},
			err:  "cannot read BATCH query type for child #0: cannot read [byte]: EOF",
		},
		{
			name: "unsupported batch query type",
			buf:  []byte{0x01, 0x00, 0x01, 0x02},
			err:  "unsupported BATCH child type for child #0: 2",
		},
		{
			name: "invalid batch query string",
			buf:  []byte{0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01},
			err:  "cannot read BATCH query string for child #0: cannot read [long string] length: cannot read [int]: unexpected EOF",
		},
		{
			name: "invalid batch query id",
			buf:  []byte{0x01, 0x00, 0x01, 0x01},
			err:  "cannot read BATCH query id for child #0: cannot read [short bytes] length: cannot read [short]: EOF",
		},
		{
			name: "invalid batch query values count",
			buf:  []byte{0x01, 0x00, 0x01, 0x01, 0x00, 0x01, 0x00},
			err:  "cannot read BATCH positional values for child #0: cannot read positional [value]s length: cannot read [short]: EOF",
		},
		{
			name: "invalid batch query values",
			buf:  []byte{0x01, 0x00, 0x01, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01},
			err:  "cannot read BATCH positional values for child #0: cannot read positional [value]s element 0 content: cannot read [value] content: EOF",
		},
		{
			name: "invalid batch query value length",
			buf:  []byte{0x01, 0x00, 0x01, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00},
			err:  "cannot read BATCH positional values for child #0: cannot read positional [value]s element 0 content: cannot read [value] length: cannot read [int]: unexpected EOF",
		},
		{
			name: "invalid batch consistency",
			buf:  []byte{0x01, 0x00, 0x01, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00},
			err:  "cannot read BATCH consistency level: cannot read [short]: EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &partialBatchCodec{}
			reader := NewFrameBodyReader(tt.buf)
			_, err := c.Decode(reader, tt.version)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestPartialBatchCodec_Encode(t *testing.T) {
	tests := []struct {
		name string
		msg  *message.Batch
	}{
		{
			name: "simple",
			msg: &message.Batch{
				Type: primitive.BatchTypeLogged,
				Children: []*message.BatchChild{
					{Query: "select * from table1"},
					{Id: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}},
				},
			},
		},
		{
			name: "positional values",
			msg: &message.Batch{
				Type: primitive.BatchTypeLogged,
				Children: []*message.BatchChild{
					{
						Query: "select * from table1",
						Values: []*primitive.Value{
							{
								Type:     primitive.ValueTypeRegular,
								Contents: []byte{0x1, 0x2, 0x3},
							},
							{
								Type:     primitive.ValueTypeRegular,
								Contents: []byte{0x4, 0x5, 0x6},
							},
						},
					},
					{
						Id: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
						Values: []*primitive.Value{
							{
								Type:     primitive.ValueTypeRegular,
								Contents: []byte{0x1, 0x2, 0x3},
							},
							{
								Type:     primitive.ValueTypeRegular,
								Contents: []byte{0x4, 0x5, 0x6},
							},
						},
					},
				},
			},
		},
		{
			name: "consistency",
			msg: &message.Batch{
				Type: primitive.BatchTypeLogged,
				Children: []*message.BatchChild{
					{Query: "select * from table1"},
					{Id: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}},
				},
				Consistency: primitive.ConsistencyLevelEachQuorum,
			},
		},
		{
			name: "parameters",
			msg: &message.Batch{
				Type: primitive.BatchTypeLogged,
				Children: []*message.BatchChild{
					{Query: "select * from table1"},
					{Id: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}},
				},
				Consistency:       primitive.ConsistencyLevelEachQuorum,
				SerialConsistency: consistencyLevelPtr(primitive.ConsistencyLevelSerial),
				DefaultTimestamp:  int64Ptr(12345),
				Keyspace:          "keyspace1",
				NowInSeconds:      int32Ptr(56789),
			},
		},
	}

	codec := &partialBatchCodec{}
	version := primitive.ProtocolVersion4

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen, err := builtinBatchCodec.EncodedLength(tt.msg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with built-in query codec")

			var expectedBytes bytes.Buffer
			err = builtinBatchCodec.Encode(tt.msg, &expectedBytes, version)
			require.NoError(t, err, "unexpected error encoding with built-in query codec")

			partialMsg, err := codec.Decode(NewFrameBodyReader(expectedBytes.Bytes()), version)
			assert.NoError(t, err, "unexpected error decoding partial query")

			batchMsg := partialMsg.(*PartialBatch)

			// Sanity checks
			assert.Equal(t, tt.msg.Type, batchMsg.Type)
			assert.Equal(t, tt.msg.Consistency, batchMsg.Consistency)
			if assert.Equal(t, len(tt.msg.Children), len(batchMsg.Queries)) {
				for i, child := range tt.msg.Children {
					if child.Query != "" {
						assert.Equal(t, child.Query, batchMsg.Queries[i].QueryOrId)
					} else if child.Id != nil {
						assert.Equal(t, child.Id, batchMsg.Queries[i].QueryOrId)
					} else {
						assert.Fail(t, "invalid batch child, must have a query or an ID")
					}
				}
			}

			actualLen, err := codec.EncodedLength(partialMsg, version)
			assert.NoError(t, err, "unexpected error calculating encoded length with partial query codec")

			assert.Equal(t, expectedLen, actualLen)

			var actualBytes bytes.Buffer
			err = codec.Encode(tt.msg, &actualBytes, version)
			require.NoError(t, err, "unexpected error encoding with partial query codec")

			assert.Equal(t, expectedBytes, actualBytes)
		})
	}
}

func int32Ptr(x int32) *int32                                                      { return &x }
func int64Ptr(x int64) *int64                                                      { return &x }
func consistencyLevelPtr(x primitive.ConsistencyLevel) *primitive.ConsistencyLevel { return &x }
