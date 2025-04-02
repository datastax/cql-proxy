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

package codecs

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var (
	builtinCodecs       = buildBuiltinCodecs()
	builtinQueryCodec   = builtinCodecs[primitive.OpCodeQuery]
	builtinExecuteCodec = builtinCodecs[primitive.OpCodeExecute]
	builtinBatchCodec   = builtinCodecs[primitive.OpCodeBatch]
)

type partialQueryCodec struct{}

func (c *partialQueryCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	switch query := msg.(type) {
	case *PartialQuery:
		if err := primitive.WriteLongString(query.Query, dest); err != nil {
			return fmt.Errorf("cannot write QUERY query string: %w", err)
		}
		if err := primitive.WriteShort(uint16(query.Consistency), dest); err != nil {
			return fmt.Errorf("cannot write QUERY consistency level: %w", err)
		}
		if _, err := dest.Write(query.Parameters); err != nil {
			return fmt.Errorf("cannot write QUERY parameters: %w", err)
		}
		return nil
	default:
		return builtinQueryCodec.Encode(msg, dest, version)
	}
}

func (c *partialQueryCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (int, error) {
	switch query := msg.(type) {
	case *PartialQuery:
		length := primitive.LengthOfLongString(query.Query)
		length += primitive.LengthOfShort
		length += len(query.Parameters)
		return length, nil
	default:
		return builtinQueryCodec.EncodedLength(msg, version)
	}
}

func (c *partialQueryCodec) Decode(source io.Reader, _ primitive.ProtocolVersion) (msg message.Message, err error) {
	var (
		query       string
		consistency uint16
	)

	reader, err := toFrameBodyReader(source)
	if err != nil {
		return nil, err
	}

	if query, err = primitive.ReadLongString(reader); err != nil {
		return nil, err
	}

	if consistency, err = primitive.ReadShort(reader); err != nil {
		return nil, fmt.Errorf("cannot read QUERY consistency level: %w", err)
	}

	return &PartialQuery{
		Query:       query,
		Consistency: primitive.ConsistencyLevel(consistency),
		Parameters:  reader.RemainingBytes(),
	}, nil
}

func (c *partialQueryCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

type PartialQuery struct {
	Query       string
	Consistency primitive.ConsistencyLevel
	Parameters  []byte // The rest of the query message
}

func (p *PartialQuery) IsResponse() bool {
	return false
}

func (p *PartialQuery) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

func (p *PartialQuery) DeepCopyMessage() message.Message {
	panic("not implemented")
}

func (p *PartialQuery) String() string {
	return "QUERY \"" + p.Query + "\""
}

type PartialExecute struct {
	QueryId          []byte
	ResultMetadataId []byte
	Consistency      primitive.ConsistencyLevel
	Parameters       []byte // The rest of the execute message
}

func (m *PartialExecute) IsResponse() bool {
	return false
}

func (m *PartialExecute) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

func (m *PartialExecute) DeepCopyMessage() message.Message {
	panic("not implemented")
}

func (m *PartialExecute) String() string {
	return "EXECUTE " + hex.EncodeToString(m.QueryId)
}

type partialExecuteCodec struct{}

func (c *partialExecuteCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	switch execute := msg.(type) {
	case *PartialExecute:
		if err := primitive.WriteShortBytes(execute.QueryId, dest); err != nil {
			return fmt.Errorf("cannot write EXECUTE query string: %w", err)
		}
		if version >= primitive.ProtocolVersion5 {
			if err := primitive.WriteShortBytes(execute.ResultMetadataId, dest); err != nil {
				return fmt.Errorf("cannot write EXECUTE consistency level: %w", err)
			}
		}
		if err := primitive.WriteShort(uint16(execute.Consistency), dest); err != nil {
			return fmt.Errorf("cannot write EXECUTE consistency level: %w", err)
		}
		if _, err := dest.Write(execute.Parameters); err != nil {
			return fmt.Errorf("cannot write EXECUTE parameters: %w", err)
		}
		return nil
	default:
		return builtinExecuteCodec.Encode(msg, dest, version)
	}
}

func (c *partialExecuteCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (size int, err error) {
	switch execute := msg.(type) {
	case *PartialExecute:
		length := primitive.LengthOfShortBytes(execute.QueryId)
		if version >= primitive.ProtocolVersion5 {
			length += primitive.LengthOfShortBytes(execute.ResultMetadataId)
		}
		length += primitive.LengthOfShort
		length += len(execute.Parameters)
		return length, nil
	default:
		return builtinExecuteCodec.EncodedLength(msg, version)
	}
}

func (c *partialExecuteCodec) Decode(source io.Reader, version primitive.ProtocolVersion) (msg message.Message, err error) {
	var (
		queryId          []byte
		resultMetadataId []byte
		consistency      uint16
	)

	reader, err := toFrameBodyReader(source)
	if err != nil {
		return nil, err
	}

	if queryId, err = primitive.ReadShortBytes(reader); err != nil {
		return nil, fmt.Errorf("cannot read EXECUTE query id: %w", err)
	} else if len(queryId) == 0 {
		return nil, errors.New("EXECUTE missing query id")
	}

	if version >= primitive.ProtocolVersion5 {
		if resultMetadataId, err = primitive.ReadShortBytes(reader); err != nil {
			return nil, fmt.Errorf("cannot read EXECUTE result metadata id: %w", err)
		}

		if len(resultMetadataId) == 0 {
			return nil, errors.New("EXECUTE missing result metadata id")
		}
	}

	if consistency, err = primitive.ReadShort(reader); err != nil {
		return nil, fmt.Errorf("cannot read EXECUTE consistency level: %w", err)
	}

	return &PartialExecute{
		QueryId:     queryId,
		Consistency: primitive.ConsistencyLevel(consistency),
		Parameters:  reader.RemainingBytes(),
	}, nil
}

func (c *partialExecuteCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

type PartialBatchQuery struct {
	QueryOrId interface{}
	Values    []byte
}

type PartialBatch struct {
	Type        primitive.BatchType
	Queries     []PartialBatchQuery
	Consistency primitive.ConsistencyLevel
	Parameters  []byte // The rest of the batch message
}

func (p PartialBatch) IsResponse() bool {
	return false
}

func (p PartialBatch) GetOpCode() primitive.OpCode {
	return primitive.OpCodeBatch
}

func (p PartialBatch) DeepCopyMessage() message.Message {
	panic("not implemented")
}

func (p PartialBatch) String() string {
	return fmt.Sprintf("BATCH (%d statements)", len(p.Queries))
}

type partialBatchCodec struct{}

func (p partialBatchCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	switch batch := msg.(type) {
	case *PartialBatch:
		if err := primitive.WriteByte(byte(batch.Type), dest); err != nil {
			return fmt.Errorf("cannot write BATCH type: %w", err)
		}
		if err := primitive.WriteShort(uint16(len(batch.Queries)), dest); err != nil {
			return fmt.Errorf("cannot write BATCH query count: %w", err)
		}
		for i, query := range batch.Queries {
			switch q := query.QueryOrId.(type) {
			case string:
				if err := primitive.WriteByte(byte(primitive.BatchChildTypeQueryString), dest); err != nil {
					return fmt.Errorf("cannot write BATCH query kind 0 for child #%d: %w", i, err)
				}
				if err := primitive.WriteLongString(q, dest); err != nil {
					return fmt.Errorf("cannot write BATCH query string for child #%d: %w", i, err)
				}
			case []byte:
				if err := primitive.WriteByte(byte(primitive.BatchChildTypePreparedId), dest); err != nil {
					return fmt.Errorf("cannot write BATCH query kind 1 for child #%d: %w", i, err)
				}
				if err := primitive.WriteShortBytes(q, dest); err != nil {
					return fmt.Errorf("cannot write BATCH query id for child #%d: %w", i, err)
				}
			}
			if _, err := dest.Write(query.Values); err != nil {
				return fmt.Errorf("cannot write BATCH positional values for child #%d: %w", i, err)
			}
		}
		if err := primitive.WriteShort(uint16(batch.Consistency), dest); err != nil {
			return fmt.Errorf("cannot write BATCH consistency: %w", err)
		}
		if _, err := dest.Write(batch.Parameters); err != nil {
			return fmt.Errorf("cannot write BATCH parameters: %w", err)
		}
		return nil
	default:
		return builtinBatchCodec.Encode(msg, dest, version)
	}
}

func (p partialBatchCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (int, error) {
	switch batch := msg.(type) {
	case *PartialBatch:
		length := primitive.LengthOfByte  // Batch type (logged, unlogged, etc.)
		length += primitive.LengthOfShort // Number of queries
		for _, query := range batch.Queries {
			length += primitive.LengthOfByte // Query kind
			switch q := query.QueryOrId.(type) {
			case string:
				length += primitive.LengthOfLongString(q)
			case []byte:
				length += primitive.LengthOfShortBytes(q)
			}
			length += len(query.Values) // Positional query parameters
		}
		length += primitive.LengthOfShort // Consistency level
		length += len(batch.Parameters)   // Remaining flags/parameters
		return length, nil
	default:
		return builtinBatchCodec.EncodedLength(msg, version)
	}
}

func (p partialBatchCodec) Decode(source io.Reader, version primitive.ProtocolVersion) (msg message.Message, err error) {
	var (
		consistency uint16
		typ         uint8
	)

	reader, err := toFrameBodyReader(source)
	if err != nil {
		return nil, err
	}

	if typ, err = primitive.ReadByte(reader); err != nil {
		return nil, fmt.Errorf("cannot read BATCH type: %w", err)
	}
	if err = primitive.CheckValidBatchType(primitive.BatchType(typ)); err != nil {
		return nil, err
	}
	var count uint16
	if count, err = primitive.ReadShort(reader); err != nil {
		return nil, fmt.Errorf("cannot read BATCH query count: %w", err)
	}
	queryOrIds := make([]PartialBatchQuery, count)
	for i := 0; i < int(count); i++ {
		var queryTyp uint8
		if queryTyp, err = primitive.ReadByte(reader); err != nil {
			return nil, fmt.Errorf("cannot read BATCH query type for child #%d: %w", i, err)
		}

		var queryOrId interface{}
		switch primitive.BatchChildType(queryTyp) {
		case primitive.BatchChildTypeQueryString:
			if queryOrId, err = primitive.ReadLongString(reader); err != nil {
				return nil, fmt.Errorf("cannot read BATCH query string for child #%d: %w", i, err)
			}
		case primitive.BatchChildTypePreparedId:
			if queryOrId, err = primitive.ReadShortBytes(reader); err != nil {
				return nil, fmt.Errorf("cannot read BATCH query id for child #%d: %w", i, err)
			}
		default:
			return nil, fmt.Errorf("unsupported BATCH child type for child #%d: %v", i, queryTyp)
		}

		pos := reader.Position()
		if err = skipPositionalValues(reader); err != nil {
			return nil, fmt.Errorf("cannot read BATCH positional values for child #%d: %w", i, err)
		}

		queryOrIds[i] = PartialBatchQuery{
			QueryOrId: queryOrId,
			Values:    reader.BytesSince(pos),
		}
	}

	if consistency, err = primitive.ReadShort(reader); err != nil {
		return nil, fmt.Errorf("cannot read BATCH consistency level: %w", err)
	}

	return &PartialBatch{
		Type:        primitive.BatchType(typ),
		Queries:     queryOrIds,
		Consistency: primitive.ConsistencyLevel(consistency),
		Parameters:  reader.RemainingBytes(),
	}, nil
}

func (p partialBatchCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeBatch
}

func skipPositionalValues(source io.Reader) error {
	if length, err := primitive.ReadShort(source); err != nil {
		return fmt.Errorf("cannot read positional [value]s length: %w", err)
	} else {
		for i := uint16(0); i < length; i++ {
			if err = skipValue(source); err != nil {
				return fmt.Errorf("cannot read positional [value]s element %d content: %w", i, err)
			}
		}
		return nil
	}
}

func skipValue(source io.Reader) error {
	if length, err := primitive.ReadInt(source); err != nil {
		return fmt.Errorf("cannot read [value] length: %w", err)
	} else if length <= 0 {
		return nil
	} else {
		if _, err = io.CopyN(ioutil.Discard, source, int64(length)); err != nil {
			return fmt.Errorf("cannot read [value] content: %w", err)
		}
		return nil
	}
}

func buildBuiltinCodecs() map[primitive.OpCode]message.Codec {
	codecs := make(map[primitive.OpCode]message.Codec)
	for _, codec := range message.DefaultMessageCodecs {
		opcode := codec.GetOpCode()
		codecs[opcode] = codec
	}
	return codecs
}

func toFrameBodyReader(source io.Reader) (*FrameBodyReader, error) {
	if reader, ok := source.(*FrameBodyReader); ok {
		return reader, nil
	} else if buf, ok := source.(*bytes.Buffer); ok {
		return NewFrameBodyReader(buf.Bytes()), nil
	}
	return nil, errors.New("source is not a frame body reader")
}
