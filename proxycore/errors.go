package proxycore

import (
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type CqlError interface {
	error
	Message() *message.Error
}

func NewCqlError(msg *message.Error) CqlError {
	return &defaultCqlError{
		msg,
	}
}

type defaultCqlError struct {
	msg *message.Error
}

func (d* defaultCqlError) Error() string {
	return fmt.Sprintf("cql error: %v", d.msg)
}

func (d* defaultCqlError) Message() *message.Error {
	return d.msg
}

