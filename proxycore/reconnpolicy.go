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

package proxycore

import (
	"math/bits"
	"math/rand"
	"time"
)

const (
	defaultBaseDelay = 2 * time.Second
	defaultMaxDelay  = 10 * time.Minute
)

type ReconnectPolicy interface {
	NextDelay() time.Duration
	Reset()
	Clone() ReconnectPolicy
}

type defaultReconnectPolicy struct {
	attempts    int
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func NewReconnectPolicy() ReconnectPolicy {
	return NewReconnectPolicyWithDelays(defaultBaseDelay, defaultMaxDelay)
}

func NewReconnectPolicyWithDelays(baseDelay, maxDelay time.Duration) ReconnectPolicy {
	return &defaultReconnectPolicy{
		attempts:    0,
		maxAttempts: calcMaxAttempts(baseDelay),
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
	}
}

func (d *defaultReconnectPolicy) NextDelay() time.Duration {
	if d.attempts >= d.maxAttempts {
		return d.maxDelay
	}
	jitter := time.Duration(rand.Intn(30)+85) * time.Millisecond
	exp := time.Millisecond << d.attempts
	d.attempts++
	delay := d.baseDelay + exp + jitter
	if delay > d.maxDelay {
		delay = d.maxDelay
	}
	return delay
}

func (d *defaultReconnectPolicy) Reset() {
	d.attempts = 0
}

func (d defaultReconnectPolicy) Clone() ReconnectPolicy {
	return NewReconnectPolicyWithDelays(d.baseDelay, d.maxDelay)
}

func calcMaxAttempts(baseDelay time.Duration) int {
	return 63 - bits.LeadingZeros64(uint64(baseDelay))
}
