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

package proxy

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRun_HealthChecks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	clusterPort, clusterAddr, proxyBindAddr, httpBindAddr := generateTestAddrs(testAddr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)
	defer cluster.Shutdown()
	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		rc := Run(ctx, []string{
			"--bind", proxyBindAddr,
			"--contact-points", clusterAddr,
			"--port", strconv.Itoa(clusterPort),
			"--health-check",
			"--http-bind", httpBindAddr,
			"--readiness-timeout", "200ms", // Use short timeout for the test
		})
		assert.Equal(t, 0, rc)
		wg.Done()
	}()

	defer func() {
		cancel()
		wg.Wait()
	}()

	require.True(t, waitUntil(10*time.Second, func() bool {
		return checkLiveness(httpBindAddr)
	}))

	// Sanity check the readiness of the cluster
	outage, status := checkReadiness(t, httpBindAddr)
	assert.Equal(t, time.Duration(0), outage)
	assert.Equal(t, http.StatusOK, status)

	// Stop only node in the cluster to simulate an outage
	cluster.Stop(1)

	// Wait for the readiness check to fail
	require.True(t, waitUntil(10*time.Second, func() bool {
		outage, status = checkReadiness(t, httpBindAddr)
		return outage > 0 && status == http.StatusServiceUnavailable
	}))

	// Restart the cluster
	err = cluster.Start(ctx, 1)
	require.NoError(t, err)

	// Wait for the readiness check to recover
	require.True(t, waitUntil(10*time.Second, func() bool {
		outage, status = checkReadiness(t, httpBindAddr)
		return outage == 0 && status == http.StatusOK
	}))
}

func TestRun_ConfigFileWithPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	proxy1Addr, proxy2Addr := testAddr, generateTestAddr(testAddr, 1)
	clusterPort, clusterAddr, proxy1BindAddr, httpBindAddr := generateTestAddrs(proxy1Addr)
	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)

	defer cluster.Shutdown()
	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string   `yaml:"rpc-address"`
		DataCenter    string   `yaml:"data-center"`
		ContactPoints []string `yaml:"contact-points"`
		HealthCheck   bool     `yaml:"health-check"`
		HttpBind      string   `yaml:"http-bind"`
		Peers         []PeerConfig
	}{
		Bind:          proxy1BindAddr,
		RPCAddr:       proxy1Addr,
		DataCenter:    "dc-1",
		Port:          clusterPort,
		ContactPoints: []string{clusterAddr},
		HealthCheck:   true,
		HttpBind:      httpBindAddr,
		Peers: []PeerConfig{{
			RPCAddr: proxy1Addr,
			DC:      "dc-1",
		}, {
			RPCAddr: proxy2Addr,
			DC:      "dc-2",
		}},
	})

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		rc := Run(ctx, []string{
			"--config", configFileName,
		})
		assert.Equal(t, 0, rc)
		wg.Done()
	}()

	defer func() {
		cancel()
		wg.Wait()
	}()

	require.True(t, waitUntil(10*time.Second, func() bool {
		return checkLiveness(httpBindAddr)
	}))

	cl := connectTestClient(t, ctx, proxy1BindAddr)

	rs, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT rpc_address, data_center, tokens FROM system.local",
	})
	require.Equal(t, rs.RowCount(), 1)

	rpcAddr, err := rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, proxy1Addr, rpcAddr.String())

	dataCenter, err := rs.Row(0).StringByName("data_center")
	require.NoError(t, err)
	assert.Equal(t, "dc-1", dataCenter)

	val, err := rs.Row(0).ByName("tokens")
	require.NoError(t, err)
	tokens := val.([]*string)
	assert.NotEmpty(t, tokens)
	assert.Equal(t, "-9223372036854775808", *tokens[0])

	rs, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT rpc_address, data_center, tokens FROM system.peers",
	})
	require.Equal(t, rs.RowCount(), 1)

	rpcAddr, err = rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, proxy2Addr, rpcAddr.String())

	dataCenter, err = rs.Row(0).StringByName("data_center")
	require.NoError(t, err)
	assert.Equal(t, "dc-2", dataCenter)

	val, err = rs.Row(0).ByName("tokens")
	require.NoError(t, err)
	tokens = val.([]*string)
	assert.NotEmpty(t, tokens)
	assert.Equal(t, "-3074457345618258602", *tokens[0])
}

func TestRun_ConfigFileWithTokensProvided(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	proxy1Addr, proxy2Addr := testAddr, generateTestAddr(testAddr, 1)
	clusterPort, clusterAddr, proxy1BindAddr, httpBindAddr := generateTestAddrs(testAddr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)
	defer cluster.Shutdown()

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string `yaml:"rpc-address"`
		DataCenter    string `yaml:"data-center"`
		Tokens        []string
		ContactPoints []string `yaml:"contact-points"`
		HealthCheck   bool     `yaml:"health-check"`
		HttpBind      string   `yaml:"http-bind"`
		Peers         []PeerConfig
	}{
		Bind:          proxy1BindAddr,
		RPCAddr:       proxy1Addr,
		DataCenter:    "dc-1",
		Tokens:        []string{"0", "1"}, // Provide custom tokens
		Port:          clusterPort,
		ContactPoints: []string{clusterAddr},
		HealthCheck:   true,
		HttpBind:      httpBindAddr,
		Peers: []PeerConfig{{
			RPCAddr: proxy2Addr,
			Tokens:  []string{"42", "613"}, // Same here
		}},
	})

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		rc := Run(ctx, []string{
			"--config", configFileName,
		})
		assert.Equal(t, 0, rc)
		wg.Done()
	}()

	defer func() {
		cancel()
		wg.Wait()
	}()

	require.True(t, waitUntil(10*time.Second, func() bool {
		return checkLiveness(httpBindAddr)
	}))

	cl := connectTestClient(t, ctx, proxy1BindAddr)

	rs, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT rpc_address, tokens FROM system.local",
	})
	require.Equal(t, rs.RowCount(), 1)

	rpcAddr, err := rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, proxy1Addr, rpcAddr.String())

	val, err := rs.Row(0).ByName("tokens")
	require.NoError(t, err)
	tokens := val.([]*string)
	assert.NotEmpty(t, tokens)
	assert.Equal(t, "0", *tokens[0])
	assert.Equal(t, "1", *tokens[1])

	rs, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT rpc_address, data_center, tokens FROM system.peers",
	})
	require.Equal(t, rs.RowCount(), 1)

	rpcAddr, err = rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, proxy2Addr, rpcAddr.String())

	val, err = rs.Row(0).ByName("tokens")
	require.NoError(t, err)
	tokens = val.([]*string)
	assert.NotEmpty(t, tokens)
	assert.Equal(t, "42", *tokens[0])
	assert.Equal(t, "613", *tokens[1])
}

func TestRun_ConfigFileWithPeersAndNoRPCAddr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy1Addr, proxy2Addr := testAddr, generateTestAddr(testAddr, 1)
	clusterPort, clusterAddr, proxy1BindAddr, _ := generateTestAddrs(proxy1Addr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)
	defer cluster.Shutdown()
	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string   `yaml:"rpc-address"`
		DataCenter    string   `yaml:"data-center"`
		ContactPoints []string `yaml:"contact-points"`
		Peers         []PeerConfig
	}{
		Bind:          proxy1BindAddr,
		ContactPoints: []string{clusterAddr},
		Port:          clusterPort,
		// No RPC address, but using peers
		Peers: []PeerConfig{{
			RPCAddr: proxy2Addr,
			DC:      "dc-2",
		}},
	})
	require.NoError(t, err)

	rc := Run(ctx, []string{
		"--config", configFileName,
	})
	require.Equal(t, 1, rc)
}

func TestRun_ConfigFileWithNoPeerTokens(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy1Addr, proxy2Addr := testAddr, generateTestAddr(testAddr, 1)
	clusterPort, clusterAddr, proxy1BindAddr, _ := generateTestAddrs(proxy1Addr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	defer cluster.Shutdown()

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string `yaml:"rpc-address"`
		DataCenter    string `yaml:"data-center"`
		Tokens        []string
		ContactPoints []string `yaml:"contact-points"`
		Peers         []PeerConfig
	}{
		Bind:          proxy1BindAddr,
		ContactPoints: []string{clusterAddr},
		Port:          clusterPort,
		RPCAddr:       proxy1Addr,
		Tokens:        []string{"0"}, // Local tokens set
		Peers: []PeerConfig{{
			RPCAddr: proxy2Addr,
			DC:      "dc-2",
			// No peer tokens
		}},
	})
	require.NoError(t, err)

	rc := Run(ctx, []string{
		"--config", configFileName,
	})
	require.Equal(t, 1, rc)
}

func TestRun_ConfigFileWithInvalidPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy1Addr := testAddr
	clusterPort, clusterAddr, proxy1BindAddr, _ := generateTestAddrs(proxy1Addr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	defer cluster.Shutdown()

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string   `yaml:"rpc-address"`
		DataCenter    string   `yaml:"data-center"`
		ContactPoints []string `yaml:"contact-points"`
		HealthCheck   bool     `yaml:"health-check"`
		HttpBind      string   `yaml:"http-bind"`
		Peers         []PeerConfig
	}{
		ContactPoints: []string{clusterAddr},
		Bind:          proxy1BindAddr,
		RPCAddr:       proxy1Addr,
		Port:          clusterPort,
		Peers: []PeerConfig{{
			RPCAddr: "", // Empty
			DC:      "dc-2",
		}},
	})
	require.NoError(t, err)

	rc := Run(ctx, []string{
		"--config", configFileName,
	})
	require.Equal(t, 1, rc)
}

func TestRun_ProxyTLS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	clusterPort, clusterAddr, proxyBindAddr, httpBindAddr := generateTestAddrs(testAddr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)
	defer cluster.Shutdown()
	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	dir, err := ioutil.TempDir("", "certs")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	certFile := path.Join(dir, "cert")
	err = ioutil.WriteFile(certFile, append(testCertPEM[:], testCAPEM[:]...), 0644)
	require.NoError(t, err)

	keyFile := path.Join(dir, "key")
	err = ioutil.WriteFile(keyFile, testKeyPEM, 0644)
	require.NoError(t, err)

	go func() {
		rc := Run(ctx, []string{
			"--bind", proxyBindAddr,
			"--contact-points", clusterAddr,
			"--port", strconv.Itoa(clusterPort),
			"--health-check",
			"--http-bind", httpBindAddr,
			"--readiness-timeout", "200ms", // Use short timeout for the test
			"--proxy-cert-file", certFile,
			"--proxy-key-file", keyFile,
		})
		assert.Equal(t, 0, rc)
		wg.Done()
	}()

	defer func() {
		cancel()
		wg.Wait()
	}()

	require.True(t, waitUntil(10*time.Second, func() bool {
		return checkLiveness(httpBindAddr)
	}))

	rootCAs, err := createCertPool()
	require.NoError(t, err)

	ok := rootCAs.AppendCertsFromPEM(testCAPEM)
	require.True(t, ok, "the provided CA cert could not be added to the root CA pool")

	cfg := &tls.Config{RootCAs: rootCAs, ServerName: "127.0.0.1"}
	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpointTLS(proxyBindAddr, cfg), proxycore.ClientConnConfig{})
	defer cl.Close()
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion4, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version)

	rs, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT * FROM system.local",
	})
	require.Equal(t, rs.RowCount(), 1)
}

func TestRun_UnsupportedWriteConsistency(t *testing.T) {
	unsupportedConsistencies := []primitive.ConsistencyLevel{
		primitive.ConsistencyLevelAny,
		primitive.ConsistencyLevelOne,
		primitive.ConsistencyLevelLocalOne,
	}

	selectConsistency := unsupportedConsistencies[0]

	checkMutationConsistency := func(consistency primitive.ConsistencyLevel) {
		for _, unsupported := range unsupportedConsistencies {
			assert.NotEqual(t, unsupported, consistency, "received unsupported consistency")
		}
		assert.Equal(t, primitive.ConsistencyLevelLocalQuorum, consistency)
	}

	checkConsistency := func(query string, consistency primitive.ConsistencyLevel) {
		if strings.Contains(query, "INSERT") {
			checkMutationConsistency(consistency)
		} else if strings.Contains(query, "SELECT") {
			assert.Equal(t, selectConsistency, consistency)
		} else {
			assert.Fail(t, "received invalid query")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	clusterPort, clusterAddr, proxyBindAddr, httpBindAddr := generateTestAddrs(testAddr)

	cluster := proxycore.NewMockCluster(net.ParseIP(testStartAddr), clusterPort)

	var prepared sync.Map

	cluster.Handlers = proxycore.NewMockRequestHandlers(proxycore.MockRequestHandlers{
		primitive.OpCodeQuery: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			if msg := cl.InterceptQuery(frm.Header, frm.Body.Message.(*message.Query)); msg != nil {
				return msg
			} else {
				query := frm.Body.Message.(*message.Query)
				checkConsistency(query.Query, query.Options.Consistency)
				return &message.RowsResult{
					Metadata: &message.RowsMetadata{
						ColumnCount: 0,
					},
					Data: message.RowSet{},
				}
			}
		},
		primitive.OpCodePrepare: func(client *proxycore.MockClient, frm *frame.Frame) message.Message {
			prepare := frm.Body.Message.(*message.Prepare)
			preparedId := md5.Sum([]byte(prepare.Query))
			prepared.Store(preparedId, prepare.Query)
			return &message.PreparedResult{
				PreparedQueryId: preparedId[:],
			}
		},
		primitive.OpCodeExecute: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			execute := frm.Body.Message.(*message.Execute)
			preparedId := preparedIdKey(execute.QueryId)
			query, ok := prepared.Load(preparedId)
			assert.True(t, ok, "unable to find prepared ID")
			checkConsistency(query.(string), execute.Options.Consistency)
			return &message.RowsResult{
				Metadata: &message.RowsMetadata{
					ColumnCount: 0,
				},
				Data: message.RowSet{},
			}
		},
		primitive.OpCodeBatch: func(cl *proxycore.MockClient, frm *frame.Frame) message.Message {
			batch := frm.Body.Message.(*message.Batch)
			checkMutationConsistency(batch.Consistency)
			return &message.VoidResult{}
		},
	})

	defer cluster.Shutdown()
	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		rc := Run(ctx, []string{
			"--bind", proxyBindAddr,
			"--contact-points", clusterAddr,
			"--port", strconv.Itoa(clusterPort),
			"--health-check",
			"--http-bind", httpBindAddr,
			"--readiness-timeout", "200ms", // Use short timeout for the test
			"--unsupported-write-consistencies", "any,one,local_one",
		})
		assert.Equal(t, 0, rc)
		wg.Done()
	}()

	defer func() {
		cancel()
		wg.Wait()
	}()

	require.True(t, waitUntil(10*time.Second, func() bool {
		return checkLiveness(httpBindAddr)
	}))

	cl, err := proxycore.ConnectClient(ctx, proxycore.NewEndpoint(proxyBindAddr), proxycore.ClientConnConfig{})
	defer cl.Close()
	require.NoError(t, err)

	version, err := cl.Handshake(ctx, primitive.ProtocolVersion4, nil)
	require.NoError(t, err)
	assert.Equal(t, primitive.ProtocolVersion4, version)

	insertPrepareResp, err := cl.SendAndReceive(
		ctx,
		frame.NewFrame(version, 0, &message.Prepare{Query: "INSERT INTO test (k, v) VALUES ('k1', 'v1')"}),
	)
	require.NoError(t, err)
	require.Equal(t, primitive.OpCodeResult, insertPrepareResp.Header.OpCode)
	insertPrepareResult, ok := insertPrepareResp.Body.Message.(*message.PreparedResult)
	assert.True(t, ok, "expected prepared result")

	selectPrepareResp, err := cl.SendAndReceive(
		ctx,
		frame.NewFrame(version, 0, &message.Prepare{Query: "SELECT * FROM test.test"}),
	)
	require.NoError(t, err)
	require.Equal(t, primitive.OpCodeResult, selectPrepareResp.Header.OpCode)
	selectPrepareResult, ok := selectPrepareResp.Body.Message.(*message.PreparedResult)
	assert.True(t, ok, "expected prepared result")

	t.Run("simple queries", func(t *testing.T) {
		for _, unsupported := range unsupportedConsistencies {
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
				Query: "INSERT INTO test (k, v) VALUES ('k1', 'v1')",
				Options: &message.QueryOptions{
					Consistency: unsupported,
				},
			})
			assert.NoError(t, err)
		}

		_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
			Query: "SELECT * FROM test",
			Options: &message.QueryOptions{
				Consistency: selectConsistency,
			},
		})
		assert.NoError(t, err)
	})

	t.Run("prepared queries", func(t *testing.T) {
		for _, unsupported := range unsupportedConsistencies {
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Execute{
				QueryId: insertPrepareResult.PreparedQueryId,
				Options: &message.QueryOptions{
					Consistency: unsupported,
				},
			})
			assert.NoError(t, err)
		}

		_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Execute{
			QueryId: selectPrepareResult.PreparedQueryId,
			Options: &message.QueryOptions{
				Consistency: selectConsistency,
			},
		})
		assert.NoError(t, err)
	})

	t.Run("batch", func(t *testing.T) {
		for _, unsupported := range unsupportedConsistencies {
			_, err = cl.Query(ctx, primitive.ProtocolVersion4, &message.Batch{
				Children: []*message.BatchChild{
					{Query: "INSERT INTO test (k, v) VALUES ('k1', 'v1')"},
					{Id: insertPrepareResp.Body.Message.(*message.PreparedResult).PreparedQueryId},
				},
				Consistency: unsupported,
			})
			assert.NoError(t, err)
		}
	})

}

func writeTempYaml(o interface{}) (name string, err error) {
	bytes, err := yaml.Marshal(o)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "cql-proxy-yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.Write(bytes)
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func checkLiveness(host string) bool {
	res, err := http.Get(fmt.Sprintf("http://%s%s", host, livenessPath))
	if err == nil {
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}
	return false
}

func checkReadiness(t *testing.T, host string) (outage time.Duration, status int) {
	res, err := http.Get(fmt.Sprintf("http://%s%s", host, readinessPath))
	require.NoError(t, err)

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)

	var ready struct {
		OutageDuration string
	}

	err = json.Unmarshal(body, &ready)
	require.NoError(t, err)

	outage, err = time.ParseDuration(ready.OutageDuration)
	require.NoError(t, err)

	return outage, res.StatusCode
}

func createCertPool() (*x509.CertPool, error) {
	ca, err := x509.SystemCertPool()
	if err != nil && runtime.GOOS == "windows" {
		return x509.NewCertPool(), nil
	}
	return ca, err
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
