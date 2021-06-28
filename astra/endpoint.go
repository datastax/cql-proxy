package astra

import (
	"cql-proxy/proxycore"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io/ioutil"
	"net/http"
	"time"
)

type astraResolver struct {
	host   string
	bundle *Bundle
}

type astraEndpoint struct {
	addr      string
	tlsConfig *tls.Config
}

func NewResolver(bundle *Bundle) proxycore.EndpointResolver {
	return &astraResolver{
		bundle: bundle,
	}
}

func (a *astraResolver) Resolve() ([]proxycore.Endpoint, error) {
	var metadata *astraMetadata

	url := fmt.Sprintf("https://%s:%d/metadata", a.bundle.Host(), a.bundle.Port())
	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: a.bundle.TLSConfig(),
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

	a.host = metadata.ContactInfo.SniProxyAddress

	var endpoints []proxycore.Endpoint
	for _, cp := range metadata.ContactInfo.ContactPoints {
		endpoints = append(endpoints, &astraEndpoint{
			addr:      metadata.ContactInfo.SniProxyAddress,
			tlsConfig: copyTLSConfig(a.bundle, cp),
		})
	}

	return endpoints, nil
}

func (a *astraResolver) Create(row proxycore.Row) (proxycore.Endpoint, error) {
	if len(a.host) != 0 {
		return nil, errors.New("host never resolved")
	}
	hostId, err := row.ByName("host_id")
	if err != nil {
		return nil, err
	}
	uuid := hostId.(primitive.UUID)
	return &astraEndpoint{
		addr:      a.host,
		tlsConfig: copyTLSConfig(a.bundle, uuid.String()),
	}, nil
}

func (a *astraEndpoint) String() string {
	return a.Key()
}

func (a *astraEndpoint) Key() string {
	return fmt.Sprintf("%s:%s", a.addr, a.tlsConfig.ServerName) // TODO: cache!!!
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
