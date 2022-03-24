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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobinLoadBalancer_NewQueryPlan(t *testing.T) {
	lb := NewRoundRobinLoadBalancer()

	qp := lb.NewQueryPlan()
	assert.Nil(t, qp.Next())

	newHost := func(addr string) *Host {
		return &Host{Endpoint: &defaultEndpoint{addr: addr}}
	}

	lb.OnEvent(&BootstrapEvent{Hosts: []*Host{newHost("127.0.0.1"), newHost("127.0.0.2"), newHost("127.0.0.3")}})
	qp = lb.NewQueryPlan()
	assert.Equal(t, newHost("127.0.0.2"), qp.Next())
	assert.Equal(t, newHost("127.0.0.3"), qp.Next())
	assert.Equal(t, newHost("127.0.0.1"), qp.Next())
	assert.Nil(t, qp.Next())

	lb.OnEvent(&AddEvent{Host: newHost("127.0.0.4")})

	qp = lb.NewQueryPlan()
	assert.Equal(t, newHost("127.0.0.3"), qp.Next())
	assert.Equal(t, newHost("127.0.0.4"), qp.Next())
	assert.Equal(t, newHost("127.0.0.1"), qp.Next())
	assert.Equal(t, newHost("127.0.0.2"), qp.Next())
	assert.Nil(t, qp.Next())

	lb.OnEvent(&RemoveEvent{Host: newHost("127.0.0.4")})

	qp = lb.NewQueryPlan()
	assert.Equal(t, newHost("127.0.0.1"), qp.Next())
	assert.Equal(t, newHost("127.0.0.2"), qp.Next())
	assert.Equal(t, newHost("127.0.0.3"), qp.Next())
	assert.Nil(t, qp.Next())

	lb.OnEvent(&RemoveEvent{Host: newHost("127.0.0.3")})

	qp = lb.NewQueryPlan()
	assert.Equal(t, newHost("127.0.0.1"), qp.Next())
	assert.Equal(t, newHost("127.0.0.2"), qp.Next())
	assert.Nil(t, qp.Next())

	lb.OnEvent(&RemoveEvent{Host: newHost("127.0.0.2")})

	qp = lb.NewQueryPlan()
	assert.Equal(t, newHost("127.0.0.1"), qp.Next())
	assert.Nil(t, qp.Next())

	lb.OnEvent(&RemoveEvent{Host: newHost("127.0.0.1")})

	qp = lb.NewQueryPlan()
	assert.Nil(t, qp.Next())
}
