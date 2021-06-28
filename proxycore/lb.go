package proxycore

import (
	"sync"
	"sync/atomic"
)

type QueryPlan interface {
	Next() *Host
}

type LoadBalancer interface {
	ClusterListener
	NewQueryPlan() QueryPlan
}

func NewRoundRobinLoadBalancer() LoadBalancer {
	return &roundRobinLoadBalancer{
		mu: &sync.Mutex{},
	}
}

type roundRobinLoadBalancer struct {
	hosts atomic.Value
	index uint32
	mu    *sync.Mutex
}

func (l *roundRobinLoadBalancer) OnEvent(event *ClusterEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch event.typ {
	case ClusterEventBootstrap:
		l.hosts.Store(event.hosts)
	case ClusterEventAdded:
		l.hosts.Store(append(l.copy(), event.host))
	case ClusterEventRemoved:
		cpy := l.copy()
		for i, h := range cpy {
			if h.Endpoint().Key() == event.host.Endpoint().Key() {
				l.hosts.Store(append(cpy[:i], cpy[i+1:]...))
				break
			}
		}
	}
}

func (l *roundRobinLoadBalancer) copy() []*Host {
	hosts := l.hosts.Load().([]*Host)
	var cpy []*Host
	copy(cpy, hosts)
	return cpy
}

func (l *roundRobinLoadBalancer) NewQueryPlan() QueryPlan {
	return &roundRobinQueryPlan{
		hosts:  l.hosts.Load().([]*Host),
		offset: atomic.AddUint32(&l.index, 1),
		index:  0,
	}
}

type roundRobinQueryPlan struct {
	hosts  []*Host
	offset uint32
	index  uint32
}

func (p *roundRobinQueryPlan) Next() *Host {
	l := uint32(len(p.hosts))
	if p.index >= l {
		return nil
	}
	host := p.hosts[(p.offset+p.index)%l]
	p.index++
	return host
}
