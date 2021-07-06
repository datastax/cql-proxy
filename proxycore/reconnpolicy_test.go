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
