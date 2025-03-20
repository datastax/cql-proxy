package proxycore

import (
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

type PartialQuery struct {
	Query string
}

func (p *PartialQuery) IsResponse() bool {
	return false
}

func (p *PartialQuery) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

func (p *PartialQuery) Clone() message.Message {
	return &PartialQuery{p.Query}
}

type PartialExecute struct {
	QueryID []byte
}

func (m *PartialExecute) IsResponse() bool {
	return false
}

func (m *PartialExecute) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

func (m *PartialExecute) Clone() message.Message {
	return &PartialExecute{
		QueryID: primitive.CloneByteSlice(m.QueryID),
	}
}

type PartialBatch struct {
	QueryOrIDs []interface{}
}

func (p PartialBatch) IsResponse() bool {
	return false
}

func (p PartialBatch) GetOpCode() primitive.OpCode {
	return primitive.OpCodeBatch
}

func (p PartialBatch) Clone() message.Message {
	queryOrIds := make([]interface{}, len(p.QueryOrIDs))
	copy(queryOrIds, p.QueryOrIDs)
	return &PartialBatch{queryOrIds}
}
