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

package proxycore

import (
	"context"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestConnectCluster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMockCluster(net.ParseIP("127.0.0.0"))

	err := c.Add(ctx, 1)
	require.NoError(t, err)

	err = c.Add(ctx, 2)
	require.NoError(t, err)

	err = c.Add(ctx, 3)
	require.NoError(t, err)

	cluster, err := ConnectCluster(ctx, ClusterConfig{
		Version:         primitive.ProtocolVersion4,
		Resolver:        NewResolver("127.0.0.1:9042"),
		ReconnectPolicy: NewReconnectPolicyWithDelays(200*time.Millisecond, time.Second),
	})
	require.NoError(t, err)

	events := make(chan interface{})

	err = cluster.Listen(ClusterListenerFunc(func(event interface{}) {
		events <- event
	}))
	require.NoError(t, err)

	wait := func() interface{} {
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-timer.C:
			require.Fail(t, "timed out waiting for event")
		case event := <-events:
			return event
		}
		require.Fail(t, "event expected")
		return nil
	}

	event := wait()
	require.IsType(t, event, &BootstrapEvent{Hosts: nil})

	c.Stop(1)
	event = wait()
	assert.Equal(t, event, &ReconnectEvent{&defaultEndpoint{addr: "127.0.0.2:9042"}})

	c.Stop(2)
	event = wait()
	assert.Equal(t, event, &ReconnectEvent{&defaultEndpoint{addr: "127.0.0.3:9042"}})

	err = c.Start(ctx, 1)
	require.NoError(t, err)

	c.Stop(3)
	event = wait()
	assert.Equal(t, event, &ReconnectEvent{&defaultEndpoint{addr: "127.0.0.1:9042"}})
}
