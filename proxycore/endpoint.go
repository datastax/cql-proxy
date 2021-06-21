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
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type Endpoint interface {
	Addr() string
	IsResolved() bool
	TlsConfig() *tls.Config
}

type defaultEndpoint struct {
	addr string
}

func (e *defaultEndpoint) IsResolved() bool {
	return true
}

func NewEndpoint(addr string) Endpoint {
	return &defaultEndpoint{addr}
}

func (e *defaultEndpoint) Addr() string {
	return e.addr
}

func (e *defaultEndpoint) TlsConfig() *tls.Config {
	return nil
}

type ContactPointResolver interface {
	Resolve() ([]Endpoint, error)
}

func NewDefaultResolver(contactPoints ...string) ContactPointResolver {
	return &defaultResolver {
		contactPoints: contactPoints,
	}
}

type defaultResolver struct {
	contactPoints []string
}

func (d *defaultResolver) Resolve() ([]Endpoint, error) {
	var endpoints []Endpoint
	for _, cp := range d.contactPoints {
		addrs, err := net.LookupHost(cp)
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			endpoints = append(endpoints, &defaultEndpoint{
				addr,
			})
		}
	}
	return endpoints, nil
}

func NewAstraResolver(bundle *Bundle) ContactPointResolver {
	return &astraResolver{
		bundle,
	}
}

func (m *astraResolver) Resolve() ([]Endpoint, error) {
	var metadata *astraMetadata

	url := fmt.Sprintf("https://%s:%d/metadata", m.bundle.Host(), m.bundle.Port())
	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: m.bundle.TLSConfig(),
		},
	}
	response, err := httpsClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("unable to get metadata from %s: %v", url, err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil,err
	}

	err = json.Unmarshal(body, &metadata)
	if err != nil {
		return nil,err
	}

	var endpoints []Endpoint
	for _, cp := range metadata.ContactInfo.ContactPoints {
		tlsConfig := m.bundle.TLSConfig()
		tlsConfig.ServerName = cp
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			certs := make([]*x509.Certificate, len(rawCerts))
			for i, asn1Data := range rawCerts {
				cert, err := x509.ParseCertificate(asn1Data)
				if err != nil {
					return errors.New("tls: failed to parse certificate from server: " + err.Error())
				}
				certs[i] = cert
			}

			opts := x509.VerifyOptions{
				Roots:         tlsConfig.RootCAs,
				CurrentTime:   time.Now(),
				DNSName:       m.bundle.Host(),
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range certs[1:] {
				opts.Intermediates.AddCert(cert)
			}
			var err error
			verifiedChains, err = certs[0].Verify(opts)
			return err
		}
		endpoints = append(endpoints, &astraEndpoint{
			addr:      metadata.ContactInfo.SniProxyAddress,
			tlsConfig: tlsConfig,
		})
	}

	return endpoints, nil
}

type astraResolver struct {
	bundle *Bundle
}

type astraEndpoint struct {
	addr string
	tlsConfig *tls.Config
}

func (a *astraEndpoint) Addr() string {
	return a.addr
}

func (a *astraEndpoint) IsResolved() bool {
	return false
}

func (a *astraEndpoint) TlsConfig() *tls.Config {
	return a.tlsConfig
}


type contactInfo struct {
	TypeName string `json:"type"`
	LocalDc string `json:"local_dc"`
	SniProxyAddress string `json:"sni_proxy_address"`
	ContactPoints []string `json:"contact_points"`
}

type astraMetadata struct {
	Version int `json:"version"`
	Region string `json:"region"`
	ContactInfo contactInfo `json:"contact_info"`
}

