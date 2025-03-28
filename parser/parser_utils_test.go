package parser

import (
	"encoding/binary"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
)

func TestPatchQueryConsistency(t *testing.T) {
	t.Run("valid QUERY body", func(t *testing.T) {
		query := []byte("SELECT * FROM users;")
		queryLen := uint32(len(query))

		body := make([]byte, 4+len(query)+2)
		binary.BigEndian.PutUint32(body[0:4], queryLen)
		copy(body[4:], query)
		binary.BigEndian.PutUint16(body[4+len(query):], uint16(primitive.ConsistencyLevelOne))

		err := PatchQueryConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		offset := 4 + len(query)
		got := binary.BigEndian.Uint16(body[offset : offset+2])
		assert.Equal(t, uint16(primitive.ConsistencyLevelQuorum), got)
	})

	t.Run("too short body", func(t *testing.T) {
		err := PatchQueryConsistency([]byte{0x01, 0x02}, primitive.ConsistencyLevelQuorum)
		assert.Error(t, err)
	})

	t.Run("not enough space for consistency", func(t *testing.T) {
		body := make([]byte, 4)
		binary.BigEndian.PutUint32(body[0:4], 10)
		err := PatchQueryConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.Error(t, err)
	})
}

func TestPatchExecuteConsistency(t *testing.T) {
	t.Run("valid EXECUTE body", func(t *testing.T) {
		id := []byte{0xCA, 0xFE, 0xBA, 0xBE}
		idLen := len(id)

		body := make([]byte, 2+idLen+2)
		binary.BigEndian.PutUint16(body[0:2], uint16(idLen))
		copy(body[2:], id)
		binary.BigEndian.PutUint16(body[2+idLen:], uint16(primitive.ConsistencyLevelLocalQuorum))

		err := PatchExecuteConsistency(body, primitive.ConsistencyLevelAll)
		assert.NoError(t, err)

		offset := 2 + idLen
		got := binary.BigEndian.Uint16(body[offset : offset+2])
		assert.Equal(t, uint16(primitive.ConsistencyLevelAll), got)
	})

	t.Run("too short body", func(t *testing.T) {
		err := PatchExecuteConsistency([]byte{0x00}, primitive.ConsistencyLevelOne)
		assert.Error(t, err)
	})

	t.Run("not enough space for consistency", func(t *testing.T) {
		body := make([]byte, 2)
		binary.BigEndian.PutUint16(body[0:2], 4)
		err := PatchExecuteConsistency(body, primitive.ConsistencyLevelOne)
		assert.Error(t, err)
	})
}
