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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"math/big"
	"testing"
	"time"
)

func TestLoadBundleZip(t *testing.T) {
	const hostname = "localhost"
	const port = 8000

	ca, err := createCA() // TODO: Cache
	require.NoError(t, err)

	path, err := writeBundle(hostname, port, ca)
	require.NoError(t, err)

	b, err := LoadBundleZip(path)
	require.NoError(t, err)

	assert.Equal(t, hostname, b.Host())
	assert.Equal(t, port, b.Port())

	// Verify CA added to cert pool
	caSub, err := asn1.Marshal(ca.cert.Subject.ToRDNSequence())
	found := false
	for _, sub := range b.TLSConfig().RootCAs.Subjects() {
		if bytes.Compare(caSub, sub) == 0 {
			found = true
		}
	}
	assert.True(t, found)

	require.Equal(t, 1, len(b.TLSConfig().Certificates))
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

func createCA() (*certPair, error) {
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

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}

	return &certPair{cert: ca, certBytes: certBytes, key: privKey}, nil
}

func createCert(ca *certPair, dnsNames []string) (*certPair, error) {
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
		DNSNames:     dnsNames,
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

	return &certPair{cert: cert, certBytes: certBytes, key: privKey}, nil
}

func writeBundle(hostname string, port int, ca *certPair) (path string, err error) {
	temp, err := ioutil.TempFile("", "bundle")
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

	config := struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}{
		hostname,
		port,
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	err = write("config.json", configJson)
	if err != nil {
		return "", err
	}

	certPEM, err := ca.certPEM()
	if err != nil {
		return "", err
	}
	err = write("ca.crt", certPEM)
	if err != nil {
		return "", err
	}

	cert, err := createCert(ca, nil) // TODO: Cache
	if err != nil {
		return "", err
	}

	certPEM, err = cert.certPEM()
	if err != nil {
		return "", err
	}
	err = write("cert", certPEM)
	if err != nil {
		return "", err
	}

	keyPEM, err := cert.keyPEM()
	if err != nil {
		return "", err
	}
	err = write("key", keyPEM)
	if err != nil {
		return "", err
	}

	return temp.Name(), z.Close()
}
