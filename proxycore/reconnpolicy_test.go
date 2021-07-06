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
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
	"time"
)

func TestDefaultReconnectPolicy(t *testing.T) {
	var tests = []struct {
		base   time.Duration
		max    time.Duration
		policy ReconnectPolicy
	}{
		{defaultBaseDelay, defaultMaxDelay, NewReconnectPolicy()},
		{time.Second, 2 * time.Minute, NewReconnectPolicyWithDelays(time.Second, 2*time.Minute)},
		{200 * time.Millisecond, time.Hour, NewReconnectPolicyWithDelays(200*time.Millisecond, time.Hour)},
		{time.Millisecond, 24 * time.Hour, NewReconnectPolicyWithDelays(time.Millisecond, 24*time.Hour)},
	}
	for _, tt := range tests {
		verifyBaseWithJitter := func(policy ReconnectPolicy) {
			assert.InDelta(t, tt.base, policy.NextDelay(), float64((85+30)*time.Millisecond)) // include jitter
		}

		iterations := int(math.Ceil(math.Log2(float64((tt.max - tt.base) / time.Millisecond))))
		verifyBaseWithJitter(tt.policy)

		for i := 0; i < iterations-1; i++ {
			tt.policy.NextDelay()
		}
		assert.Equal(t, tt.max, tt.policy.NextDelay())
		assert.Equal(t, tt.max, tt.policy.NextDelay()) // after max it should stay max

		verifyBaseWithJitter(tt.policy.Clone()) // cloned policy should be reset

		tt.policy.Reset()
		verifyBaseWithJitter(tt.policy)
	}
}
