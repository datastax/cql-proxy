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
	"testing"
	"time"

	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const (
	testProxyHTTPBind = "127.0.0.1:8001"
)

func TestRun_HealthChecks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster := proxycore.NewMockCluster(net.ParseIP(testClusterStartIP), testClusterPort)

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	defer cluster.Shutdown()

	go func() {
		rc := Run(ctx, []string{
			"--contact-points", testClusterContactPoint,
			"--port", strconv.Itoa(testClusterPort),
			"--health-check",
			"--http-bind", testProxyHTTPBind,
			"--readiness-timeout", "200ms", // Use short timeout for the test
		})
		require.Equal(t, 0, rc)
	}()

	waitUntil(10*time.Second, func() bool {
		res, err := http.Get(fmt.Sprintf("http://%s%s", testProxyHTTPBind, livenessPath))
		return err == nil && res.StatusCode == http.StatusOK
	})

	// Sanity check the readiness of the cluster
	outage, status := checkReadiness(t)
	assert.Equal(t, time.Duration(0), outage)
	assert.Equal(t, http.StatusOK, status)

	// Stop only node in the cluster to simulate an outage
	cluster.Stop(1)

	// Wait for the readiness check to fail
	waitUntil(10*time.Second, func() bool {
		outage, status = checkReadiness(t)
		return outage > 0 && status == http.StatusServiceUnavailable
	})

	// Restart the cluster
	err = cluster.Start(ctx, 1)
	require.NoError(t, err)

	// Wait for the readiness check to recover
	waitUntil(10*time.Second, func() bool {
		outage, status = checkReadiness(t)
		return outage == 0 && status == http.StatusOK
	})
}

func TestRun_ConfigFileWithPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster := proxycore.NewMockCluster(net.ParseIP(testClusterStartIP), testClusterPort)

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
		Bind:          "127.0.0.1:9042",
		RPCAddr:       "127.0.0.1",
		DataCenter:    "dc-1",
		Port:          testClusterPort,
		ContactPoints: []string{testClusterContactPoint},
		HealthCheck:   true,
		HttpBind:      testProxyHTTPBind,
		Peers: []PeerConfig{{
			RPCAddr: "127.0.0.1",
			DC:      "dc-1",
		}, {
			RPCAddr: "127.0.0.2",
			DC:      "dc-2",
		}},
	})

	go func() {
		rc := Run(ctx, []string{
			"--config", configFileName,
		})
		require.Equal(t, 0, rc)
	}()

	waitUntil(10*time.Second, func() bool {
		res, err := http.Get(fmt.Sprintf("http://%s%s", testProxyHTTPBind, livenessPath))
		return err == nil && res.StatusCode == http.StatusOK
	})

	cl := connectTestClient(t, ctx)

	rs, err := cl.Query(ctx, primitive.ProtocolVersion4, &message.Query{
		Query: "SELECT rpc_address, data_center, tokens FROM system.local",
	})
	require.Equal(t, rs.RowCount(), 1)

	rpcAddress, err := rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rpcAddress.String())

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

	rpcAddress, err = rs.Row(0).InetByName("rpc_address")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.2", rpcAddress.String())

	dataCenter, err = rs.Row(0).StringByName("data_center")
	require.NoError(t, err)
	assert.Equal(t, "dc-2", dataCenter)

	val, err = rs.Row(0).ByName("tokens")
	require.NoError(t, err)
	tokens = val.([]*string)
	assert.NotEmpty(t, tokens)
	assert.Equal(t, "-3074457345618258602", *tokens[0])
}

func TestRun_ConfigFileWithPeersAndNoRPCAddr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster := proxycore.NewMockCluster(net.ParseIP(testClusterStartIP), testClusterPort)

	err := cluster.Add(ctx, 1)
	require.NoError(t, err)

	defer cluster.Shutdown()

	configFileName, err := writeTempYaml(struct {
		Bind          string
		Port          int
		RPCAddr       string   `yaml:"rpc-address"`
		DataCenter    string   `yaml:"data-center"`
		ContactPoints []string `yaml:"contact-points"`
		Peers         []PeerConfig
	}{
		ContactPoints: []string{testClusterContactPoint},
		Port:          testClusterPort,
		Bind:          "127.0.0.1:9042",
		// No RPC address, but using peers
		Peers: []PeerConfig{{
			RPCAddr: "127.0.0.2",
			DC:      "dc-2",
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

	cluster := proxycore.NewMockCluster(net.ParseIP(testClusterStartIP), testClusterPort)

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
		ContactPoints: []string{"127.0.0.1"},
		Bind:          "127.0.0.1:9042",
		RPCAddr:       "127.0.0.1",
		Port:          testClusterPort,
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

func checkReadiness(t *testing.T) (outage time.Duration, status int) {
	res, err := http.Get(fmt.Sprintf("http://%s%s", testProxyHTTPBind, readinessPath))
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
