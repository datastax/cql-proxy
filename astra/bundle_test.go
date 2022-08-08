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
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"io"
	"io/ioutil"
	"testing"

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

	block, _ := pem.Decode(testCAPEM)
	ca, _ := x509.ParseCertificate(block.Bytes)

	// Verify CA added to cert pool
	caSub, err := asn1.Marshal(ca.Subject.ToRDNSequence())
	found := false
	for _, sub := range b.TLSConfig.RootCAs.Subjects() {
		if bytes.Compare(caSub, sub) == 0 {
			found = true
		}
	}
	assert.True(t, found)
	require.Equal(t, 1, len(b.TLSConfig.Certificates))
}

func TestLoadBundleZip_InvalidJson(t *testing.T) {
	path, err := writeBundleBytes([]byte("{"), testCAPEM, testCertPEM, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	assert.Error(t, err)
}

func TestLoadBundleZip_NoConfigJson(t *testing.T) {
	path, err := writeBundleBytes(nil, testCAPEM, testCertPEM, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'config.json' file")
	}
}

func TestLoadBundleZip_NoCACert(t *testing.T) {
	path, err := writeBundleBytes([]byte("{}"), nil, testCertPEM, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'ca.crt' file")
	}
}

func TestLoadBundleZip_NoCert(t *testing.T) {
	path, err := writeBundleBytes([]byte("{}"), testCAPEM, nil, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'cert' file")
	}
}

func TestLoadBundleZip_NoKey(t *testing.T) {
	path, err := writeBundleBytes([]byte("{}"), testCAPEM, testKeyPEM, nil)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bundle missing 'key' file")
	}
}

func TestLoadBundleZip_InvalidCACert(t *testing.T) {
	// Passing key instead of cert for 'ca.crt' file
	path, err := writeBundleBytes([]byte("{}"), testKeyPEM, testKeyPEM, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "the provided CA cert could not be added to the root CA pool")
	}
}

func TestLoadBundleZip_InvalidCert(t *testing.T) {
	// Passing key instead of cert for 'cert' file
	path, err := writeBundleBytes([]byte("{}"), testCAPEM, testKeyPEM, testKeyPEM)
	require.NoError(t, err)

	_, err = LoadBundleZipFromPath(path)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "tls: failed to find certificate PEM data in certificate input, but did find a private key")
	}
}

func TestLoadBundleZip_InvalidKey(t *testing.T) {
	// Passing cert instead of key for 'key' file
	path, err := writeBundleBytes([]byte("{}"), testCAPEM, testCertPEM, testCertPEM)
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

	return writeBundleBytes(configJson, testCAPEM, testCertPEM, testKeyPEM)
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

// Data for a test certificate pair and the corresponding CA.
// CN: localhost, IP: 127.0.0.1 Expiration: 2032/04/29

var testKeyPEM = []byte(`
-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`)

var testCertPEM = []byte(`
-----BEGIN CERTIFICATE-----
MIIF0DCCA7igAwIBAgICBnowDQYJKoZIhvcNAQELBQAwdzELMAkGA1UEBhMCVVMx
CzAJBgNVBAgTAkNBMRQwEgYDVQQHEwtTYW50YSBDbGFyYTEcMBoGA1UECRMTMzk3
NSBGcmVlZG9tIENpcmNsZTEOMAwGA1UEERMFOTUwNTQxFzAVBgNVBAoTDkRhdGFT
dGF4LCBpbmMuMB4XDTIyMDQyOTE4MTYyMVoXDTMyMDQyOTE4MTYyMVowdzELMAkG
A1UEBhMCVVMxCzAJBgNVBAgTAkNBMRQwEgYDVQQHEwtTYW50YSBDbGFyYTEcMBoG
A1UECRMTMzk3NSBGcmVlZG9tIENpcmNsZTEOMAwGA1UEERMFOTUwNTQxFzAVBgNV
BAoTDkRhdGFTdGF4LCBpbmMuMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKC
AgEAsZjsUojrggVN9qJ2+uC0R8celNRl88Fw0fsvPazY7s0C9qj+c9FhlS+KJznt
w5DeC+eWm50x+Pjj+2nCzO85iUGty491beSIt/0WECwZkK/TuCrrF8p3c1c2LkZI
WLsTk6KHAKQKA6J7flrIE1JTYCeLnOnusTGX/Y9/hCTxwPQziFdCH3Slet3iDTN5
ABcG46mIEoUo0sBvEQf2X1ZHpMdgNyxWDcYIkXraSZP6/Rap1hCaCswe68Ti17Z1
Vf1pIJXSrPyeyOAyt/oFGxcO4H1PTq0iLFB+XmlpgSNPawEDDgQztt3mMrAyvY4P
rnYNg34aftxSCWxmYqKvqD9dDZPS3cRkm04I7hA83JtiVj3/pvEtsY28Jf4ldimV
DcI3GgeEjLE0KJd0v0Y9wwS2iPDxDqFsKNun6mOTbyG5jPI794Sv/4Lr2jmOknTp
63lZyb400AVr5ThqL7dDESganHZT2dziziqNmGNdDC+t4b1RSN/SNGrYT9cn1XBP
gJj6IY0VGFTF/IIAhfAU36g2BZ4PXNyv/l61kp5AmyyMAF2JSKuknQIVKVd50paj
IBT34BFzSxVyS2XM8i+fEQtwdwTV/xhtq1qczSr1r25ChHUQrCGNewQpG59oTDUz
nvnbY4ydbruYXwvxEngO92uX4I3wUiUbHyo7IsjEAZ5aRisCAwEAAaNmMGQwDgYD
VR0PAQH/BAQDAgeAMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAOBgNV
HQ4EBwQFAQIDBAYwIwYDVR0RBBwwGoIAhwR/AAABhxAAAAAAAAAAAAAAAAAAAAAB
MA0GCSqGSIb3DQEBCwUAA4ICAQCCBQ31mkX5ejdtAmRQJD6gYYJtDJztmiX2xuzr
PPs8Q/NhxHG3JYdk2yiSmU3Jq0WjPsNyAU/XWJ3UnnMD5JhcEUENA8saTmOldFde
MhfeIQAyd+KZtj2KT1oiQalBjSRXMggV57YcMoWDYFUzGOY2ecog548FvKeoOKOo
5ajic8p+hYHjkz8TM+3wZ4wzygj8i7XvD+Hhob8sdU+oTxgIJoV431PaCwxn8lHT
oXHTD1UsGCXm/Supkq3oLB5OfuWE0JSrAaA3Nndt4PnK9kisG1cX8e99OrR/c8eV
JEUsSZxOC4ftjMtGs0J/+DBQs4RTi4+VhHM5xo6HerCLR5/kH2hjxqtnhNFbbev3
4/yb8KPTO3XVf03rJBFlmjjfToTcmNjE8rSDcGtB0/XcyWUYn3fmWntmJbrIVHyF
nkmm2/ZHAMJfIYFxniwF1KAfqMkJsY49ziS0WjjU9VvD7sGSR7KzJFSVc31eIjBf
0hy3NdkgS73JSQo4C61lyIi2w4L02rSn2Gh/b3J26xxxpPVML96uXGFWDpZEJtOR
DqJzOELCZQrh+HKtzauG/fuSa+SpfSC9/aeVh64JkfJmdNN/0yINOO3STUs5YibG
QhZVrqVrfwPNosy/TfhoU8kE8xI9JchbKh5MAg8+rDQRtZ0Lyt8a0rvYTA/EvxrV
i8aCxg==
-----END CERTIFICATE-----
`)

var testCAPEM = []byte(`
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
`)
