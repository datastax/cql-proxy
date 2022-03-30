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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/datastax/cql-proxy/proxycore"
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
