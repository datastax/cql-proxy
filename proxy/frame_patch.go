package proxy

import (
	"encoding/binary"
	"fmt"

	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// patchQueryConsistency modifies the consistency level of a QUERY message in-place
// by locating the consistency field directly in the frame body.
//
// Layout based on the CQL native protocol v4 spec:
// /* <query: long string><consistency: short><flags: byte>... */
func patchQueryConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	if len(body) < 6 {
		return fmt.Errorf("body too short for QUERY")
	}

	queryLen := binary.BigEndian.Uint32(body[0:4])
	offset := 4 + int(queryLen)

	if len(body) < offset+2 {
		return fmt.Errorf("not enough bytes to patch QUERY consistency")
	}

	// Modify the batch consistency field
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(newConsistency))

	return nil
}

// patchExecuteConsistency modifies the consistency level of an EXECUTE message in-place
// by locating the consistency field directly after the prepared statement ID.
//
// Layout based on the CQL native protocol v4 spec:
// /* <id: short bytes><consistency: short><flags: byte>... */
func patchExecuteConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	if len(body) < 2 {
		return fmt.Errorf("body too short for EXECUTE")
	}

	idLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + idLen

	if len(body) < offset+2 {
		return fmt.Errorf("not enough bytes to patch EXECUTE consistency")
	}

	// Modify the batch consistency field
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(newConsistency))

	return nil
}

// patchBatchConsistency modifies the consistency level of a BATCH message in-place
// by locating and modifying the consistency field of the batch, which applies to all queries in the batch.
//
// Layout based on the CQL native protocol v4 spec:
// /* <type: byte>
//
//	<n: short>             (number of queries)
//	<queries...>           (queries themselves)
//	<batch consistency: short>
//	<flags: byte>
//	[<serial_consistency: short>]
//	[<timestamp: long>] */
func patchBatchConsistency(body []byte, newConsistency primitive.ConsistencyLevel) error {
	if len(body) < 7 {
		// Not enough bytes for even the basic batch layout (at least 2 bytes for n + 2 bytes for consistency + 1 byte for flags)
		return fmt.Errorf("invalid batch body: too short")
	}

	offset := 3 // <type> [byte] <n> [short] (3 bytes)
	numQueries := binary.BigEndian.Uint16(body[1:3])

	// Process the queries
	for i := uint16(0); i < numQueries; i++ {
		if len(body) <= offset {
			return fmt.Errorf("query #%d exceeds body length", i)
		}

		queryType := body[offset]
		offset++ // Move past the query <kind> [byte]

		switch primitive.BatchChildType(queryType) {
		case primitive.BatchChildTypeQueryString:
			queryLength := binary.BigEndian.Uint32(body[offset : offset+4])
			offset += 4 // Move past the length
			_ = body[offset : offset+int(queryLength)]
			offset += int(queryLength)
		case primitive.BatchChildTypePreparedId:
			stmtIDLength := binary.BigEndian.Uint16(body[offset : offset+2])
			offset += 2 // Move past the length
			_ = body[offset : offset+int(stmtIDLength)]
			offset += int(stmtIDLength)
		default:
			return fmt.Errorf("unsupported BATCH child type for query #%d: %v", i, queryType)
		}

		// Skip positional values
		if err := skipPositionalValuesByteSlice(body, &offset); err != nil {
			return fmt.Errorf("cannot skip positional values for query #%d: %w", i, err)
		}
	}

	// Patch consistency at the right spot
	if len(body) < offset+2 {
		return fmt.Errorf("not enough bytes to patch consistency")
	}
	binary.BigEndian.PutUint16(body[offset:], uint16(newConsistency))

	return nil
}

// skipPositionalValuesByteSlice skips the positional values in the byte slice
// It reads the length of positional values and skips them based on the byte slice offset.
func skipPositionalValuesByteSlice(body []byte, offset *int) error {
	if len(body) <= *offset+2 {
		return fmt.Errorf("insufficient bytes to read positional values length")
	}
	length := binary.BigEndian.Uint16(body[*offset : *offset+2])
	*offset += 2 // Move the offset past the length

	for i := uint16(0); i < length; i++ {
		if err := skipValueByteSlice(body, offset); err != nil {
			return fmt.Errorf("cannot skip positional value %d: %w", i, err)
		}
	}
	return nil
}

// skipValueByteSlice skips a single positional value based on its length in the byte slice.
func skipValueByteSlice(body []byte, offset *int) error {
	if len(body) <= *offset+4 {
		return fmt.Errorf("insufficient bytes to read value length")
	}
	length := int32(binary.BigEndian.Uint32(body[*offset : *offset+4]))
	*offset += 4 // Move the offset past the length

	if length == -1 || length == -2 {
		// It's a null or unset, nothing to skip
		return nil
	}
	if length < 0 {
		return fmt.Errorf("invalid negative length: %d", length)
	}

	if length > 0 {
		if len(body) < *offset+int(length) {
			return fmt.Errorf("insufficient bytes to skip value content")
		}
		*offset += int(length) // Move the offset past the value content
	}
	return nil
}
