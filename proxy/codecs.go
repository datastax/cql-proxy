package proxy

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io"
)

var codec = frame.NewCodec(&partialQueryCodec{}, &partialExecuteCodec{})

type partialQueryCodec struct{}

func (c *partialQueryCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	panic("not implemented")
}

func (c *partialQueryCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (int, error) {
	panic("not implemented")
}

func (c *partialQueryCodec) Decode(source io.Reader, version primitive.ProtocolVersion) (message.Message, error) {
	if query, err := primitive.ReadLongString(source); err != nil {
		return nil, err
	} else if query == "" {
		return nil, fmt.Errorf("cannot read QUERY empty query string")
	} else {
		return &partialQuery{query}, nil
	}
}

func (c *partialQueryCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

type partialQuery struct {
	query string
}

func (p *partialQuery) IsResponse() bool {
	return false
}

func (p *partialQuery) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

func (p *partialQuery) Clone() message.Message {
	return &partialQuery{p.query}
}

type partialExecute struct {
	queryId []byte
}

func (m *partialExecute) IsResponse() bool {
	return false
}

func (m *partialExecute) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

func (m *partialExecute) Clone() message.Message {
	return &partialExecute{
		queryId: primitive.CloneByteSlice(m.queryId),
	}
}

func (m *partialExecute) String() string {
	return "EXECUTE " + hex.EncodeToString(m.queryId)
}

type partialExecuteCodec struct{}

func (c *partialExecuteCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	panic("not implemented")
}

func (c *partialExecuteCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (size int, err error) {
	panic("not implemented")
}

func (c *partialExecuteCodec) Decode(source io.Reader, version primitive.ProtocolVersion) (msg message.Message, err error) {
	var execute = &partialExecute{}
	if execute.queryId, err = primitive.ReadShortBytes(source); err != nil {
		return nil, fmt.Errorf("cannot read EXECUTE query id: %w", err)
	} else if len(execute.queryId) == 0 {
		return nil, errors.New("EXECUTE missing query id")
	}
	return execute, nil
}

func (c *partialExecuteCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}
