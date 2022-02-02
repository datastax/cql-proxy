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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
