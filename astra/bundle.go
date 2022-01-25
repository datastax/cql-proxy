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
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"time"

	"github.com/datastax/astra-client-go/v2/astra"
)

const URL = "https://api.astra.datastax.com"

type Bundle struct {
	tlsConfig *tls.Config
	host      string
	port      int
}

func LoadBundleZip(reader *zip.Reader) (*Bundle, error) {
	contents, err := extract(reader)
	if err != nil {
		return nil, err
	}

	config := struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}{}
	err = json.Unmarshal(contents["config.json"], &config)
	if err != nil {
		return nil, err
	}

	rootCAs, err := createCertPool()
	if err != nil {
		return nil, err
	}

	ok := rootCAs.AppendCertsFromPEM(contents["ca.crt"])
	if !ok {
		return nil, fmt.Errorf("the provided CA cert could not be added to the root CA pool")
	}

	cert, err := tls.X509KeyPair(contents["cert"], contents["key"])
	if err != nil {
		return nil, err
	}

	return &Bundle{
		tlsConfig: &tls.Config{
			RootCAs:      rootCAs,
			Certificates: []tls.Certificate{cert},
			ServerName:   config.Host,
		},
		host: config.Host,
		port: config.Port,
	}, nil
}

func LoadBundleZipFromPath(path string) (*Bundle, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}

	defer func(reader *zip.ReadCloser) {
		_ = reader.Close()
	}(reader)

	return LoadBundleZip(&reader.Reader)
}

func LoadBundleZipFromURL(url, databaseID, token string, timeout time.Duration) (*Bundle, error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
	defer cancel()

	credsURL, err := generateSecureBundleURLWithResponse(url, databaseID, token, ctx)
	if err != nil {
		return nil, fmt.Errorf("error generating secure bundle zip URLs: %v", err)
	}
	resp, err := http.Get(credsURL.DownloadURL)

	defer resp.Body.Close()

	body, err := readAllWithTimeout(resp.Body, ctx)
	if err != nil {
		return nil, fmt.Errorf("error downloading bundle zip: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("error creating zip reader for bundle zip: %v", err)
	}

	return LoadBundleZip(reader)
}

func readAllWithTimeout(r io.Reader, ctx context.Context) (bytes []byte, err error) {
	ch := make(chan struct{})

	go func() {
		bytes, err = ioutil.ReadAll(r)
		close(ch)
	}()

	select {
	case <-ch:
	case <-ctx.Done():
		return nil, errors.New("timeout reading data")
	}

	return bytes, err
}

func generateSecureBundleURLWithResponse(url, databaseID, token string, ctx context.Context) (*astra.CredsURL, error) {
	client, err := astra.NewClientWithResponses(url, func(c *astra.Client) error {
		c.RequestEditors = append(c.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return nil
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	res, err := client.GenerateSecureBundleURLWithResponse(ctx, astra.DatabaseIdParam(databaseID))

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unable to generate bundle urls, failed with status code %d", res.StatusCode())
	}

	return res.JSON200, nil
}

func (b *Bundle) Host() string {
	return b.host
}

func (b *Bundle) Port() int {
	return b.port
}

func (b *Bundle) TLSConfig() *tls.Config {
	return b.tlsConfig.Clone()
}

func extract(reader *zip.Reader) (map[string][]byte, error) {
	contents := make(map[string][]byte)

	for _, file := range reader.File {
		switch file.Name {
		case "config.json", "cert", "key", "ca.crt":
			bytes, err := loadBytes(file)
			if err != nil {
				return nil, err
			}
			contents[file.Name] = bytes
		}
	}

	for _, file := range []string{"config.json", "cert", "key", "ca.crt"} {
		if _, ok := contents[file]; !ok {
			return nil, fmt.Errorf("bundle missing '%s' file", file)
		}
	}

	return contents, nil
}

func loadBytes(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func(r io.ReadCloser) {
		_ = r.Close()
	}(r)
	return ioutil.ReadAll(r)
}

func createCertPool() (*x509.CertPool, error) {
	ca, err := x509.SystemCertPool()
	if err != nil && runtime.GOOS == "windows" {
		return x509.NewCertPool(), nil
	}
	return ca, err
}
