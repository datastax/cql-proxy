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

package astra

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/datastax/cql-proxy/proxycore"
)

type astraResolver struct {
	sniProxyAddress string
	region          string
	bundle          *Bundle
	mu              *sync.Mutex
	timeout         time.Duration
}

type astraEndpoint struct {
	addr      string
	key       string
	tlsConfig *tls.Config
}

func NewResolver(bundle *Bundle, timeout time.Duration) proxycore.EndpointResolver {
	return &astraResolver{
		bundle:  bundle,
		mu:      &sync.Mutex{},
		timeout: timeout,
	}
}

func (r *astraResolver) Resolve(ctx context.Context) ([]proxycore.Endpoint, error) {
	var metadata *astraMetadata

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: r.bundle.TlsConfig.Clone(),
		},
	}

	url := fmt.Sprintf("https://%s:%d/metadata", r.bundle.Host, r.bundle.Port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	response, err := httpsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get metadata from %s: %w", url, err)
	}

	body, err := readAllWithTimeout(response.Body, ctx)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &metadata)
	if err != nil {
		return nil, err
	}

	sniProxyAddress := metadata.ContactInfo.SniProxyAddress

	r.mu.Lock()
	r.sniProxyAddress = sniProxyAddress
	r.region = metadata.Region
	r.mu.Unlock()

	var endpoints []proxycore.Endpoint
	for _, cp := range metadata.ContactInfo.ContactPoints {
		endpoints = append(endpoints, &astraEndpoint{
			addr:      sniProxyAddress,
			key:       fmt.Sprintf("%s:%s", sniProxyAddress, cp),
			tlsConfig: copyTLSConfig(r.bundle, cp),
		})
	}

	return endpoints, nil
}

func (r *astraResolver) getSNIProxyAddressAndRegion() (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sniProxyAddress) == 0 {
		return "", "", errors.New("SNI proxy address (and region) never resolved")
	}
	return r.sniProxyAddress, r.region, nil
}

func (r *astraResolver) NewEndpoint(row proxycore.Row) (proxycore.Endpoint, error) {
	sniProxyAddress, region, err := r.getSNIProxyAddressAndRegion()
	if err != nil {
		return nil, err
	}
	dc, err := row.StringByName("data_center")
	if err != nil {
		return nil, err
	}
	if len(region) > 0 && region != dc {
		return nil, proxycore.IgnoreEndpoint
	}
	hostId, err := row.UUIDByName("host_id")
	if err != nil {
		return nil, err
	} else {
		return &astraEndpoint{
			addr:      sniProxyAddress,
			key:       fmt.Sprintf("%s:%s", sniProxyAddress, &hostId),
			tlsConfig: copyTLSConfig(r.bundle, hostId.String()),
		}, nil
	}
}

func (a astraEndpoint) String() string {
	return a.Key()
}

func (a astraEndpoint) Key() string {
	return a.key
}

func (a astraEndpoint) Addr() string {
	return a.addr
}

func (a astraEndpoint) IsResolved() bool {
	return false
}

func (a astraEndpoint) TlsConfig() *tls.Config {
	return a.tlsConfig
}

func copyTLSConfig(bundle *Bundle, serverName string) *tls.Config {
	tlsConfig := bundle.TlsConfig.Clone()
	tlsConfig.ServerName = serverName
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
			DNSName:       bundle.Host,
			Intermediates: x509.NewCertPool(),
		}
		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		var err error
		verifiedChains, err = certs[0].Verify(opts)
		return err
	}
	return tlsConfig
}

type contactInfo struct {
	TypeName        string   `json:"type"`
	LocalDc         string   `json:"local_dc"`
	SniProxyAddress string   `json:"sni_proxy_address"`
	ContactPoints   []string `json:"contact_points"`
}

type astraMetadata struct {
	Version     int         `json:"version"`
	Region      string      `json:"region"`
	ContactInfo contactInfo `json:"contact_info"`
}
