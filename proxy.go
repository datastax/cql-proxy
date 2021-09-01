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

	"cql-proxy/astra"
	"cql-proxy/proxy"
	"cql-proxy/proxycore"

	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

var cli struct {
	Bundle        string   `help:"Path to secure connect bundle" short:"b" env:"BUNDLE"`
	Username      string   `help:"Username to use for authentication" short:"u" env:"USERNAME"`
	Password      string   `help:"Password to use for authentication" short:"p" env:"PASSWORD"`
	ContactPoints []string `help:"Contact points for cluster. Ignored if using the bundle path option." short:"c" env:"CONTACT_POINTS"`
	Bind          string   `help:"Address to use to bind serve" short:"a" env:"BIND"`
	Debug         bool     `help:"Show debug logging" env:"DEBUG"`
	Profiling     bool     `help:"Enable profiling" env:"PROFILING"`
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
		Version:         primitive.ProtocolVersion4,
		Resolver:        resolver,
		ReconnectPolicy: proxycore.NewReconnectPolicy(),
		NumConns:        1,
		Auth:            auth,
		Logger:          logger,
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
