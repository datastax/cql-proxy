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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBundleZip(t *testing.T) {
	const hostname = "localhost"
	const port = 8000

	path, err := writeBundle(hostname, port)
	require.NoError(t, err)

	b, err := LoadBundleZipFromPath(path)
	require.NoError(t, err)

	assert.Equal(t, hostname, b.Host)
	assert.Equal(t, port, b.Port)

	ca, err := getOrCreateCA()
	require.NoError(t, err)

	// Verify CA added to cert pool
	caSub, err := asn1.Marshal(ca.cert.Subject.ToRDNSequence())
	found := false
	for _, sub := range b.TlsConfig.RootCAs.Subjects() {
		if bytes.Compare(caSub, sub) == 0 {
			found = true
		}
	}
	assert.True(t, found)
	require.Equal(t, 1, len(b.TlsConfig.Certificates))
}

func TestLoadBundleZip_InvalidJson(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	path, err := writeBundleBytes([]byte("{"), check(ca.certPEM()), check(cert.certPEM()), check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	assert.Error(t, err)
}

func TestLoadBundleZip_NoConfigJson(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	path, err := writeBundleBytes(nil, check(ca.certPEM()), check(cert.certPEM()), check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'config.json' file")
	}
}

func TestLoadBundleZip_NoCACert(t *testing.T) {
	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	path, err := writeBundleBytes([]byte("{}"), nil, check(cert.certPEM()), check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'ca.crt' file")
	}
}

func TestLoadBundleZip_NoCert(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	path, err := writeBundleBytes([]byte("{}"), check(ca.certPEM()), nil, check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'cert' file")
	}
}

func TestLoadBundleZip_NoKey(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	path, err := writeBundleBytes([]byte("{}"), check(ca.certPEM()), check(cert.certPEM()), nil)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'key' file")
	}
}

func TestLoadBundleZip_InvalidCACert(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	// Passing key instead of cert for 'ca.crt' file
	path, err := writeBundleBytes([]byte("{}"), check(ca.keyPEM()), check(cert.certPEM()), check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "the provided CA cert could not be added to the root CA pool")
	}
}

func TestLoadBundleZip_InvalidCert(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	// Passing key instead of cert for 'cert' file
	path, err := writeBundleBytes([]byte("{}"), check(ca.certPEM()), check(cert.keyPEM()), check(cert.keyPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "tls: failed to find certificate PEM data in certificate input, but did find a private key")
	}
}

func TestLoadBundleZip_InvalidKey(t *testing.T) {
	ca, err := getOrCreateCA()
	require.NoError(t, err)

	cert, err := getOrCreateCert("")
	require.NoError(t, err)

	check := func(b []byte, err error) []byte {
		require.NoError(t, err)
		return b
	}

	// Passing cert instead of key for 'key' file
	path, err := writeBundleBytes([]byte("{}"), check(ca.certPEM()), check(cert.certPEM()), check(cert.certPEM()))
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "tls: found a certificate rather than a key in the PEM for the private key")
	}
}

type certPair struct {
	cert      *x509.Certificate
	certBytes []byte
	key       *rsa.PrivateKey
}

func (p certPair) certPEM() ([]byte, error) {
	var certPEMBytes bytes.Buffer
	err := pem.Encode(&certPEMBytes, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: p.certBytes,
	})
	return certPEMBytes.Bytes(), err
}

func (p certPair) keyPEM() ([]byte, error) {
	var keyPEMBytes bytes.Buffer
	err := pem.Encode(&keyPEMBytes, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(p.key),
	})
	return keyPEMBytes.Bytes(), err
}

var getOrCreateCA = func() func() (*certPair, error) {
	var once sync.Once
	var cached *certPair
	var err error
	return func() (*certPair, error) {
		once.Do(func() {
			ca := &x509.Certificate{
				SerialNumber: big.NewInt(2019),
				Subject: pkix.Name{
					Organization:  []string{"DataStax, inc."},
					Country:       []string{"US"},
					Province:      []string{"CA"},
					Locality:      []string{"Santa Clara"},
					StreetAddress: []string{"3975 Freedom Circle"},
					PostalCode:    []string{"95054"},
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().AddDate(10, 0, 0),
				IsCA:                  true,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
				BasicConstraintsValid: true,
			}

			var privKey *rsa.PrivateKey
			privKey, err = rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				return
			}

			var certBytes []byte
			certBytes, err = x509.CreateCertificate(rand.Reader, ca, ca, &privKey.PublicKey, privKey)
			if err != nil {
				return
			}

			cached = &certPair{cert: ca, certBytes: certBytes, key: privKey}
		})
		return cached, err
	}
}()

var getOrCreateCert = func() func(dnsName string) (*certPair, error) {
	cache := make(map[string]*certPair)
	var mu sync.Mutex
	return func(dnsName string) (*certPair, error) {
		mu.Lock()
		defer mu.Unlock()
		if cached, ok := cache[dnsName]; ok {
			return cached, nil
		} else {
			ca, err := getOrCreateCA()
			if err != nil {
				return nil, err
			}

			cert := &x509.Certificate{
				SerialNumber: big.NewInt(1658),
				Subject: pkix.Name{
					Organization:  []string{"DataStax, inc."},
					Country:       []string{"US"},
					Province:      []string{"CA"},
					Locality:      []string{"Santa Clara"},
					StreetAddress: []string{"3975 Freedom Circle"},
					PostalCode:    []string{"95054"},
				},
				IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
				DNSNames:     []string{dnsName},
				NotBefore:    time.Now(),
				NotAfter:     time.Now().AddDate(10, 0, 0),
				SubjectKeyId: []byte{1, 2, 3, 4, 6},
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
				KeyUsage:     x509.KeyUsageDigitalSignature,
			}

			privKey, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				return nil, err
			}

			certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca.cert, &privKey.PublicKey, ca.key)
			if err != nil {
				return nil, err
			}

			pair := &certPair{cert: cert, certBytes: certBytes, key: privKey}
			cache[dnsName] = pair
			return pair, nil
		}
	}
}()

func writeBundle(hostname string, port int) (path string, err error) {
	configJson, err := json.Marshal(struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}{
		hostname,
		port,
	})
	if err != nil {
		return "", err
	}

	ca, err := getOrCreateCA()
	if err != nil {
		return "", err
	}

	cert, err := getOrCreateCert("")
	if err != nil {
		return "", err
	}

	caCertPEM, err := ca.certPEM()
	if err != nil {
		return "", err
	}

	certPEM, err := cert.certPEM()
	if err != nil {
		return "", err
	}

	keyPEM, err := cert.keyPEM()
	if err != nil {
		return "", err
	}

	return writeBundleBytes(configJson, caCertPEM, certPEM, keyPEM)
}

func writeBundleBytes(configJson []byte, caCrt []byte, cert []byte, key []byte) (path string, err error) {
	temp, err := ioutil.TempFile("", "bundle.zip")
	if err != nil {
		return "", err
	}

	z := zip.NewWriter(temp)

	write := func(name string, contents []byte) error {
		var w io.Writer
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(contents)
		return err
	}

	if configJson != nil {
		err = write("config.json", configJson)
		if err != nil {
			return "", err
		}
	}

	if caCrt != nil {
		err = write("ca.crt", caCrt)
		if err != nil {
			return "", err
		}
	}

	if cert != nil {
		err = write("cert", cert)
		if err != nil {
			return "", err
		}
	}

	if key != nil {
		err = write("key", key)
		if err != nil {
			return "", err
		}
	}

	err = z.Close()
	if err != nil {
		return "", err
	}

	return temp.Name(), temp.Close()
}
