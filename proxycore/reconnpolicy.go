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
