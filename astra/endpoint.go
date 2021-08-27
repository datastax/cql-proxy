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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/primitive"

	"cql-proxy/proxycore"
)

type astraResolver struct {
	sniProxyAddress string
	bundle          *Bundle
	mu              *sync.Mutex
}

type astraEndpoint struct {
	addr      string
	key       string
	tlsConfig *tls.Config
}

func NewResolver(bundle *Bundle) proxycore.EndpointResolver {
	return &astraResolver{
		bundle: bundle,
		mu:     &sync.Mutex{},
	}
}

func (r *astraResolver) Resolve() ([]proxycore.Endpoint, error) {
	var metadata *astraMetadata

	url := fmt.Sprintf("https://%s:%d/metadata", r.bundle.Host(), r.bundle.Port())
	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: r.bundle.TLSConfig(),
		},
	}
	response, err := httpsClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("unable to get metadata from %s: %v", url, err)
	}

	body, err := ioutil.ReadAll(response.Body)
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

func (r *astraResolver) getSNIProxyAddress() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sniProxyAddress) == 0 {
		return "", errors.New("SNI proxy address never resolved")
	}
	return r.sniProxyAddress, nil
}

func (r *astraResolver) NewEndpoint(row proxycore.Row) (proxycore.Endpoint, error) {
	sniProxyAddress, err := r.getSNIProxyAddress()
	if err != nil {
		return nil, err
	}
	hostId, err := row.ByName("host_id")
	if err != nil {
		return nil, err
	}
	uuid := hostId.(primitive.UUID)
	return &astraEndpoint{
		addr:      sniProxyAddress,
		key:       fmt.Sprintf("%s:%s", sniProxyAddress, &uuid),
		tlsConfig: copyTLSConfig(r.bundle, uuid.String()),
	}, nil
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
	tlsConfig := bundle.TLSConfig()
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
			DNSName:       bundle.Host(),
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
