package proxycore

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRoundRobinLoadBalancer_NewQueryPlan(t *testing.T) {
	lb := NewRoundRobinLoadBalancer()

	qp := lb.NewQueryPlan()
	assert.Nil(t, qp.Next())

	newHost := func(addr string) *Host {
		return &Host{endpoint: &defaultEndpoint{addr: addr}}
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
