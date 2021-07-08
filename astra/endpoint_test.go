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
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"net/http"
	"testing"
)

func TestAstraResolver_Resolve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const sniProxyAddr = "localhost:8080"
	contactPoints := []string{
		"a2e24181-d732-402a-ab06-894a8b2f6094",
		"ce00ba58-a377-4022-ba09-00394ee66cfb",
		"9e339fe3-2bf2-45ce-a660-76951f39a8e8",
	}

	err := runTestMetaSvcAsync(ctx, sniProxyAddr, contactPoints)
	require.NoError(t, err)

	path, err := writeBundle("127.0.0.1", 8080)
	require.NoError(t, err)

	bundle, err := LoadBundleZipFromPath(path)
	require.NoError(t, err)

	resolver := NewResolver(bundle)

	endpoints, err := resolver.Resolve()
	require.NoError(t, err)

	for _, endpoint := range endpoints {
		assert.False(t, endpoint.IsResolved())
		assert.Equal(t, sniProxyAddr, endpoint.Addr())
		assert.Contains(t, contactPoints, endpoint.TlsConfig().ServerName)
	}
}

func runTestMetaSvcAsync(ctx context.Context, sniProxyAddr string, contactPoints []string) error {
	host, _, err := net.SplitHostPort(sniProxyAddr)
	if err != nil {
		return err
	}

	tlsConfig, err := createServerTLSConfig(host)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", sniProxyAddr)

	go func() {
		serv := &http.Server{
			Addr:      sniProxyAddr,
			TLSConfig: tlsConfig,
		}

		http.HandleFunc("/metadata", func(writer http.ResponseWriter, request *http.Request) {
			res, err := json.Marshal(astraMetadata{
				Version: 1,
				Region:  "us-east1",
				ContactInfo: contactInfo{
					SniProxyAddress: sniProxyAddr,
					ContactPoints:   contactPoints,
				},
			})
			if err != nil {
				writer.WriteHeader(500)
			} else {
				_, _ = writer.Write(res)
			}
		})

		go func() {
			select {
			case <-ctx.Done():
				_ = serv.Close()
			}
		}()

		_ = serv.ServeTLS(listener, "", "")
	}()

	return nil
}
func createServerTLSConfig(dnsName string) (*tls.Config, error) {
	serverCert, err := getOrCreateCert(dnsName)
	if err != nil {
		return nil, err
	}

	caCert, err := getOrCreateCA()
	if err != nil {
		return nil, err
	}

	rootCAs, err := createCertPool()
	if err != nil {
		return nil, err
	}

	caCertPEM, err := caCert.certPEM()
	if err != nil {
		return nil, err
	}

	if !rootCAs.AppendCertsFromPEM(caCertPEM) {
		return nil, errors.New("unable to add cert to CA pool")
	}

	serverCertPEM, err := serverCert.certPEM()
	if err != nil {
		return nil, err
	}

	serverKeyPEM, err := serverCert.keyPEM()
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      rootCAs,
		ClientCAs:    rootCAs,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}
