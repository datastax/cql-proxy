// Copyright 2020 DataStax
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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Endpoint interface {
	fmt.Stringer
	Addr() string
	IsResolved() bool
	TlsConfig() *tls.Config
	Key() string
}

type defaultEndpoint struct {
	addr string
}

func (e *defaultEndpoint) String() string {
	return e.Key()
}

func (e *defaultEndpoint) Key() string {
	return e.addr
}

func (e *defaultEndpoint) IsResolved() bool {
	return true
}

func (e *defaultEndpoint) Addr() string {
	return e.addr
}

func (e *defaultEndpoint) TlsConfig() *tls.Config {
	return nil
}

type EndpointResolver interface {
	Resolve() ([]Endpoint, error)
	Create(row Row) (Endpoint, error)
}

type defaultEndpointResolver struct {
	contactPoints []string
	defaultPort   int
}

func NewResolver(contactPoints ...string) EndpointResolver {
	return NewResolverWithDefaultPort(contactPoints, 9042)
}

func NewResolverWithDefaultPort(contactPoints []string, defaultPort int) EndpointResolver {
	return &defaultEndpointResolver{
		contactPoints: contactPoints,
		defaultPort:   defaultPort,
	}
}

func (d *defaultEndpointResolver) Resolve() ([]Endpoint, error) {
	var endpoints []Endpoint
	for _, cp := range d.contactPoints {
		parts := strings.Split(cp, ":")
		addrs, err := net.LookupHost(parts[0])
		if err != nil {
			return nil, fmt.Errorf("unable to resolve contact point %s: %v", cp, err)
		}

		port := d.defaultPort
		if len(parts) > 1 {
			port, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("contact point %s has invalid port: %v", cp, err)
			}
		}
		for _, addr := range addrs {
			endpoints = append(endpoints, &defaultEndpoint{
				fmt.Sprintf("%s:%d", addr, port),
			})
		}
	}
	return endpoints, nil
}

func (d *defaultEndpointResolver) Create(row Row) (Endpoint, error) {
	peer, err := row.ByName("peer")
	if err != nil && !errors.Is(err, ColumnNameNotFound) {
		return nil, err
	}
	rpcAddress, err := row.ByName("rpc_address")
	if err != nil {
		return nil, err
	}

	addr := rpcAddress.(net.IP)

	if addr.Equal(net.IPv4zero) || addr.Equal(net.IPv6zero) {
		addr = peer.(net.IP)
	}

	return &defaultEndpoint{
		addr: fmt.Sprintf("%s:%d", addr, d.defaultPort),
	}, nil
}
