package proxycore

import (
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

type RetryDecision int

func (r RetryDecision) String() string {
	switch r {
	case RetrySame:
		return "retry same node"
	case RetryNext:
		return "retry next node"
	case ReturnError:
		return "returning error"
	}
	return "unknown"
}

const (
	RetrySame RetryDecision = iota
	RetryNext
	ReturnError
)

type RetryPolicy interface {
	OnReadTimeout(msg *message.ReadTimeout, retryCount int) RetryDecision
	OnWriteTimeout(msg *message.WriteTimeout, retryCount int) RetryDecision
	OnUnavailable(msg *message.Unavailable, retryCount int) RetryDecision
	OnErrorResponse(msg message.Error, retryCount int) RetryDecision
}

type defaultRetryPolicy struct{}

var defaultRetryPolicyInstance defaultRetryPolicy

func NewDefaultRetryPolicy() RetryPolicy {
	return &defaultRetryPolicyInstance
}

func (d defaultRetryPolicy) OnReadTimeout(msg *message.ReadTimeout, retryCount int) RetryDecision {
	if retryCount == 0 && msg.Received >= msg.BlockFor && !msg.DataPresent {
		return RetrySame
	} else {
		return ReturnError
	}
}

func (d defaultRetryPolicy) OnWriteTimeout(msg *message.WriteTimeout, retryCount int) RetryDecision {
	if retryCount == 0 && msg.WriteType == primitive.WriteTypeBatchLog {
		return RetrySame
	} else {
		return ReturnError
	}
}

func (d defaultRetryPolicy) OnUnavailable(_ *message.Unavailable, retryCount int) RetryDecision {
	if retryCount == 0 {
		return RetryNext
	} else {
		return ReturnError
	}
}

func (d defaultRetryPolicy) OnErrorResponse(msg message.Error, retryCount int) RetryDecision {
	code := msg.GetErrorCode()
	if code == primitive.ErrorCodeReadFailure || code == primitive.ErrorCodeWriteFailure {
		return ReturnError
	} else {
		return RetryNext
	}
}
