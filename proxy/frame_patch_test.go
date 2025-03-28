package proxy

import (
	"encoding/binary"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPatchQueryConsistency(t *testing.T) {
	t.Run("valid QUERY body", func(t *testing.T) {
		query := []byte("SELECT * FROM users;")
		queryLen := uint32(len(query))

		body := make([]byte, 4+len(query)+2)
		binary.BigEndian.PutUint32(body[0:4], queryLen)
		copy(body[4:], query)
		binary.BigEndian.PutUint16(body[4+len(query):], uint16(primitive.ConsistencyLevelOne))

		err := patchQueryConsistency(body, primitive.ConsistencyLevelQuorum)
		assert.NoError(t, err)

		offset := 4 + len(query)
		got := binary.BigEndian.Uint16(body[offset : offset+2])
		assert.Equal(t, uint16(primitive.ConsistencyLevelQuorum), got)
	})

	t.Run("too short body", func(t *testing.T) {
		err := patchQueryConsistency([]byte{0x01, 0x02}, primitive.ConsistencyLevelQuorum)
		assert.Error(t, err)
	})

	t.Run("not enough space for consistency", func(t *testing.T) {
		body := make([]byte, 4)
		binary.BigEndian.PutUint32(body[0:4], 10)
		err := patchQueryConsistency(body, primitive.ConsistencyLevelQuorum)
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
		binary.BigEndian.PutUint16(body[2+idLen:], uint16(primitive.ConsistencyLevelOne))

		err := patchExecuteConsistency(body, primitive.ConsistencyLevelLocalQuorum)
		assert.NoError(t, err)

		offset := 2 + idLen
		got := binary.BigEndian.Uint16(body[offset : offset+2])
		assert.Equal(t, uint16(primitive.ConsistencyLevelLocalQuorum), got)
	})

	t.Run("too short body", func(t *testing.T) {
		err := patchExecuteConsistency([]byte{0x00}, primitive.ConsistencyLevelOne)
		assert.Error(t, err)
	})

	t.Run("not enough space for consistency", func(t *testing.T) {
		body := make([]byte, 2)
		binary.BigEndian.PutUint16(body[0:2], 4)
		err := patchExecuteConsistency(body, primitive.ConsistencyLevelOne)
		assert.Error(t, err)
	})
}

func TestPatchBatchConsistency(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		expectedOffset int
	}{
		{
			name: "valid BATCH body without flags",
			body: []byte{
				0x00, 0x00, 0x02,

				// QUERY 1: query string
				0x00,
				0x00, 0x00, 0x00, 0x13,
				'S', 'E', 'L', 'E', 'C', 'T', ' ', '*', ' ',
				'F', 'R', 'O', 'M', ' ', 'u', 's', 'e', 'r', 's',
				0x00, 0x00, // no values

				// QUERY 2: prepared
				0x01,
				0x00, 0x04,
				0xCA, 0xFE, 0xCA, 0xFE,
				0x00, 0x00, // no values

				// CONSISTENCY LEVEL
				0x00, 0x01, // ONE
			},
			// Consistency starts after:
			// - 3 bytes header
			// - 26 bytes for query 1
			// - 9 bytes for query 2
			expectedOffset: 38,
		},
		{
			name: "valid BATCH body with flags",
			body: []byte{
				// BATCH HEADER
				0x00, 0x00, 0x02,

				// QUERY 1: query string
				0x00,
				0x00, 0x00, 0x00, 0x13,
				'S', 'E', 'L', 'E', 'C', 'T', ' ', '*', ' ',
				'F', 'R', 'O', 'M', ' ', 'u', 's', 'e', 'r', 's',
				0x00, 0x00, // no values

				// QUERY 2: prepared
				0x01,
				0x00, 0x04,
				0xCA, 0xFE, 0xCA, 0xFE,
				0x00, 0x00, // no values

				// CONSISTENCY LEVEL
				0x00, 0x01, // ONE

				// FLAGS
				0x03, // WITH_SERIAL_CONSISTENCY | WITH_DEFAULT_TIMESTAMP

				// SERIAL CONSISTENCY
				0x00, 0x06, // LOCAL_SERIAL

				// TIMESTAMP
				0x00, 0x00, 0x00, 0x00, 0x3B, 0x9A, 0xCA, 0x00,
			},
			expectedOffset: 38, // still 38; flags and extras come after
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := patchBatchConsistency(tt.body, primitive.ConsistencyLevelQuorum)
			assert.NoError(t, err)

			got := binary.BigEndian.Uint16(tt.body[tt.expectedOffset:])
			assert.Equal(t, uint16(primitive.ConsistencyLevelQuorum), got)
		})
	}
}
