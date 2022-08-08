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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
)

var IgnoreEndpoint = errors.New("ignore endpoint")

type Endpoint interface {
	fmt.Stringer
	Addr() string
	IsResolved() bool
	TLSConfig() *tls.Config
	Key() string
}

type defaultEndpoint struct {
	addr      string
	tlsConfig *tls.Config
}

func (e defaultEndpoint) String() string {
	return e.Key()
}

func (e defaultEndpoint) Key() string {
	return e.addr
}

func (e defaultEndpoint) IsResolved() bool {
	return true
}

func (e defaultEndpoint) Addr() string {
	return e.addr
}

func (e defaultEndpoint) TLSConfig() *tls.Config {
	return e.tlsConfig
}

type EndpointResolver interface {
	Resolve(ctx context.Context) ([]Endpoint, error)
	NewEndpoint(row Row) (Endpoint, error)
}

type defaultEndpointResolver struct {
	contactPoints []string
	defaultPort   string
}

func NewEndpoint(addr string) Endpoint {
	return &defaultEndpoint{addr: addr}
}

func NewEndpointTLS(addr string, cfg *tls.Config) Endpoint {
	return &defaultEndpoint{addr, cfg}
}

func NewResolver(contactPoints ...string) EndpointResolver {
	return NewResolverWithDefaultPort(contactPoints, 9042)
}

func NewResolverWithDefaultPort(contactPoints []string, defaultPort int) EndpointResolver {
	return &defaultEndpointResolver{
		contactPoints: contactPoints,
		defaultPort:   strconv.Itoa(defaultPort),
	}
}

func (r *defaultEndpointResolver) Resolve(ctx context.Context) ([]Endpoint, error) {
	var endpoints []Endpoint
	var resolver net.Resolver
	for _, cp := range r.contactPoints {
		host, port, err := net.SplitHostPort(cp)
		if err != nil {
			host = cp
		}
		addrs, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve contact point %s: %v", cp, err)
		}
		if len(port) == 0 {
			port = r.defaultPort
		}
		for _, addr := range addrs {
			endpoints = append(endpoints, &defaultEndpoint{
				addr: net.JoinHostPort(addr, port),
			})
		}
	}
	return endpoints, nil
}

func (r *defaultEndpointResolver) NewEndpoint(row Row) (Endpoint, error) {
	peer, err := row.ByName("peer")
	if err != nil && !errors.Is(err, ColumnNameNotFound) {
		return nil, err
	}
	rpcAddress, err := row.InetByName("rpc_address")
	if err != nil {
		return nil, fmt.Errorf("ignoring host because its `rpc_address` is not set or is invalid: %w", err)
	}

	addr := rpcAddress
	if addr.Equal(net.IPv4zero) || addr.Equal(net.IPv6zero) {
		var ok bool
		if addr, ok = peer.(net.IP); !ok {
			return nil, errors.New("ignoring host because its `peer` is not set or is invalid")
		}
	}

	return &defaultEndpoint{
		addr: net.JoinHostPort(addr.String(), r.defaultPort),
	}, nil
}

func LookupEndpoint(endpoint Endpoint) (string, error) {
	if endpoint.IsResolved() {
		return endpoint.Addr(), nil
	} else {
		host, port, err := net.SplitHostPort(endpoint.Addr())
		if err != nil {
			return "'", err
		}
		addrs, err := net.LookupHost(host)
		if err != nil {
			return "", err
		}
		addr := addrs[rand.Intn(len(addrs))]
		if len(port) > 0 {
			addr = net.JoinHostPort(addr, port)
		}
		return addr, nil
	}
}
