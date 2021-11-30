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

package main

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"time"

	"github.com/datastax/cql-proxy/astra"
	"github.com/datastax/cql-proxy/proxy"
	"github.com/datastax/cql-proxy/proxycore"

	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

var cli struct {
	Bundle             string        `help:"Path to secure connect bundle" short:"b" env:"BUNDLE"`
	Username           string        `help:"Username to use for authentication" short:"u" env:"USERNAME"`
	Password           string        `help:"Password to use for authentication" short:"p" env:"PASSWORD"`
	ContactPoints      []string      `help:"Contact points for cluster. Ignored if using the bundle path option." short:"c" env:"CONTACT_POINTS"`
	ProtocolVersion    string        `help:"Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" short:"n" env:"PROTOCOL_VERSION"`
	MaxProtocolVersion string        `help:"Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" short:"m" env:"MAX_PROTOCOL_VERSION"`
	Bind               string        `help:"Address to use to bind serve" short:"a" env:"BIND"`
	Debug              bool          `help:"Show debug logging" env:"DEBUG"`
	Profiling          bool          `help:"Enable profiling" env:"PROFILING"`
	HeartbeatInterval  time.Duration `help:"Interval between performing heartbeats to the cluster" default:"30s" env:"HEARTBEAT_INTERVAL"`
	IdleTimeout        time.Duration `help:"Time between successful heartbeats before a connection to the cluster is considered unresponsive and closed" default:"60s" env:"IDLE_TIMEOUT"`
}

func parseProtocolVersion(s string) (version primitive.ProtocolVersion, ok bool) {
	ok = true
	lowered := strings.ToLower(s)
	if lowered == "3" || lowered == "v3" {
		version = primitive.ProtocolVersion3
	} else if lowered == "4" || lowered == "v4" {
		version = primitive.ProtocolVersion4
	} else if lowered == "5" || lowered == "v5" {
		version = primitive.ProtocolVersion5
	} else if lowered == "65" || lowered == "dsev1" {
		version = primitive.ProtocolVersionDse1
	} else if lowered == "66" || lowered == "dsev2" {
		version = primitive.ProtocolVersionDse1
	} else {
		ok = false
	}
	return version, ok
}

func main() {
	cliCtx := kong.Parse(&cli)

	var resolver proxycore.EndpointResolver

	if len(cli.Bundle) > 0 {
		bundle, err := astra.LoadBundleZipFromPath(cli.Bundle)
		if err != nil {
			cliCtx.Fatalf("unable to open bundle %s: %v", cli.Bundle, err)
		}
		resolver = astra.NewResolver(bundle)
	} else if len(cli.ContactPoints) > 0 {
		resolver = proxycore.NewResolver(cli.ContactPoints...)
	} else {
		cliCtx.Fatalf("must provide either bundle path or contact points")
	}

	if cli.HeartbeatInterval >= cli.IdleTimeout {
		cliCtx.Fatalf("idle-timeout must be greater than heartbeat-interval")
	}

	version := primitive.ProtocolVersion4
	if len(cli.ProtocolVersion) > 0 {
		var ok bool
		if version, ok = parseProtocolVersion(cli.ProtocolVersion); !ok {
			cliCtx.Fatalf("unsupported protocol version: %s", cli.ProtocolVersion)
		}
	}

	maxVersion := primitive.ProtocolVersion4
	if len(cli.MaxProtocolVersion) > 0 {
		var ok bool
		if maxVersion, ok = parseProtocolVersion(cli.MaxProtocolVersion); !ok {
			cliCtx.Fatalf("unsupported max protocol version: %s", cli.ProtocolVersion)
		}
	}

	if version > maxVersion {
		cliCtx.Fatalf("default protocol version is greater than max protocol version")
	}

	ctx := context.Background()

	var logger *zap.Logger
	var err error
	if cli.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		cliCtx.Fatalf("unable to create logger")
	}

	var auth proxycore.Authenticator

	if len(cli.Username) > 0 || len(cli.Password) > 0 {
		auth = proxycore.NewPasswordAuth(cli.Username, cli.Password)
	}

	p := proxy.NewProxy(ctx, proxy.Config{
		Version:           version,
		MaxVersion:        maxVersion,
		Resolver:          resolver,
		ReconnectPolicy:   proxycore.NewReconnectPolicy(),
		NumConns:          1,
		Auth:              auth,
		Logger:            logger,
		HeartBeatInterval: cli.HeartbeatInterval,
		IdleTimeout:       cli.IdleTimeout,
	})

	bind, _, err := net.SplitHostPort(cli.Bind)
	if err != nil {
		bind = net.JoinHostPort(cli.Bind, "9042")
	}

	if cli.Profiling {
		go func() {
			err := http.ListenAndServe("localhost:6060", nil) // Profiling
			if err != nil {
				logger.Error("unable to setup profiling", zap.Error(err))
			}
		}()
	}

	err = p.ListenAndServe(bind)
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}
}
