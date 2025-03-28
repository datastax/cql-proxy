package proxy

import (
	"bytes"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const version = primitive.ProtocolVersion4

func TestPatchQueryConsistencyValid(t *testing.T) {
	var queryCodec message.Codec

	for _, c := range message.DefaultMessageCodecs {
		if c.GetOpCode() == primitive.OpCodeQuery {
			queryCodec = c
		}
	}
	assert.NotNil(t, queryCodec)

	t.Run("valid QUERY body", func(t *testing.T) {
		var buf bytes.Buffer
		err := queryCodec.Encode(&message.Query{
			Query: "SELECT * FROM test",
			Options: &message.QueryOptions{
				Consistency: primitive.ConsistencyLevelOne,
			},
		}, &buf, version)
		assert.NoError(t, err)

		body := buf.Bytes()
		err = patchQueryConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		msg, err := queryCodec.Decode(bytes.NewBuffer(body), version)
		require.NoError(t, err)

		assert.Equal(t, primitive.ConsistencyLevelQuorum, msg.(*message.Query).Options.Consistency)
	})
}

func TestPatchExecuteConsistencyValid(t *testing.T) {

	localSerialConsistency := primitive.ConsistencyLevelLocalSerial

	var queryCodec message.Codec

	for _, c := range message.DefaultMessageCodecs {
		if c.GetOpCode() == primitive.OpCodeExecute {
			queryCodec = c
		}
	}
	assert.NotNil(t, queryCodec)

	t.Run("valid EXECUTE body", func(t *testing.T) {
		var buf bytes.Buffer

		msg := &message.Execute{
			QueryId:          []byte{0x0a, 0x0b, 0x0c},
			ResultMetadataId: []byte{0x0d, 0x0e, 0x0f},
			Options: &message.QueryOptions{
				Consistency:       primitive.ConsistencyLevelOne,
				SerialConsistency: &localSerialConsistency,
			},
		}

		err := queryCodec.Encode(msg, &buf, version)
		assert.NoError(t, err)

		body := buf.Bytes()
		err = patchExecuteConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		mesg, err := queryCodec.Decode(bytes.NewBuffer(body), version)
		require.NoError(t, err)

		assert.Equal(t, primitive.ConsistencyLevelQuorum, mesg.(*message.Execute).Options.Consistency)
	})

}

func TestPatchBatchConsistencyValid(t *testing.T) {
	localSerialConsistency := primitive.ConsistencyLevelLocalSerial
	timestamp := int64(1234567890)

	var queryCodec message.Codec

	for _, c := range message.DefaultMessageCodecs {
		if c.GetOpCode() == primitive.OpCodeBatch {
			queryCodec = c
		}
	}
	assert.NotNil(t, queryCodec)

	t.Run("valid Batch patch consistency with values", func(t *testing.T) {
		var buf bytes.Buffer

		msgWithFlags := &message.Batch{
			Type: primitive.BatchTypeLogged,
			Children: []*message.BatchChild{
				{
					Query: "SELECT * FROM table WHERE id = ?",
					Values: []*primitive.Value{
						{Type: primitive.ValueTypeRegular, Contents: []byte{0x01, 0x02, 0x03}},
					},
				},
				{
					Id: []byte{0x01, 0x02, 0x03},
					Values: []*primitive.Value{
						{Type: primitive.ValueTypeNull},
					},
				},
			},
			Consistency:       primitive.ConsistencyLevelOne,
			SerialConsistency: &localSerialConsistency,
			DefaultTimestamp:  &timestamp,
		}

		err := queryCodec.Encode(msgWithFlags, &buf, version)
		assert.NoError(t, err)

		body := buf.Bytes()
		err = patchBatchConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		mesg, err := queryCodec.Decode(bytes.NewBuffer(body), version)
		require.NoError(t, err)

		assert.Equal(t, primitive.ConsistencyLevelQuorum, mesg.(*message.Batch).Consistency)
	})

	t.Run("valid Batch patch consistency without values", func(t *testing.T) {
		var buf bytes.Buffer

		msgWithFlags := &message.Batch{
			Type: primitive.BatchTypeLogged,
			Children: []*message.BatchChild{
				{
					Query: "SELECT * FROM table WHERE id = ?",
				},
				{
					Id: []byte{0x01, 0x02, 0x03},
				},
			},
			Consistency:       primitive.ConsistencyLevelOne,
			SerialConsistency: &localSerialConsistency,
			DefaultTimestamp:  &timestamp,
		}

		err := queryCodec.Encode(msgWithFlags, &buf, version)
		assert.NoError(t, err)

		body := buf.Bytes()
		err = patchBatchConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		mesg, err := queryCodec.Decode(bytes.NewBuffer(body), version)
		require.NoError(t, err)

		assert.Equal(t, primitive.ConsistencyLevelQuorum, mesg.(*message.Batch).Consistency)
	})

}
