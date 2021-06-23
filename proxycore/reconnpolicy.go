package proxycore

import (
	"math/rand"
	"time"
)

type ReconnectPolicy interface {
	NextDelay() time.Duration
	Reset()
	Clone() ReconnectPolicy
}

type defaultReconnectPolicy struct {
	attempts  int
	baseDelay time.Duration
	maxDelay  time.Duration
}

func NewReconnectPolicy() ReconnectPolicy {
	return NewReconnectPolicyWithDelays(2*time.Second, 10*time.Minute)
}

func NewReconnectPolicyWithDelays(baseDelay, maxDelay time.Duration) ReconnectPolicy {
	return defaultReconnectPolicy{
		attempts:  0,
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

func (d defaultReconnectPolicy) NextDelay() time.Duration {
	jitter := time.Duration(rand.Intn(30)+85) * time.Millisecond
	d.attempts++
	delay := d.baseDelay + (time.Millisecond << d.attempts) + jitter
	if delay > d.maxDelay {
		delay = d.maxDelay
	}
	return delay
}

func (d defaultReconnectPolicy) Reset() {
	d.attempts = 0
}

func (d defaultReconnectPolicy) Clone() ReconnectPolicy {
	return NewReconnectPolicyWithDelays(d.baseDelay, d.maxDelay)
}
