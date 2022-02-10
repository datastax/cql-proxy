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
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/datastax/cql-proxy/astra"
	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

const livenessPath = "/liveness"
const readinessPath = "/readiness"

var cli struct {
	AstraBundle        string        `help:"Path to secure connect bundle for an Astra database. Requires '--username' and '--password'. Ignored if using the token or contact points option." short:"b" env:"ASTRA_BUNDLE"`
	AstraToken         string        `help:"Token used to authenticate to an Astra database. Requires '--astra-database-id'. Ignored if using the bundle path or contact points option." short:"t" env:"ASTRA_TOKEN"`
	AstraDatabaseID    string        `help:"Database ID of the Astra database. Requires '--astra-token'" short:"i" env:"ASTRA_DATABASE_ID"`
	AstraAPIURL        string        `help:"URL for the Astra API" default:"https://api.astra.datastax.com" env:"ASTRA_API_URL"`
	ContactPoints      []string      `help:"Contact points for cluster. Ignored if using the bundle path or token option." short:"c" env:"CONTACT_POINTS"`
	Username           string        `help:"Username to use for authentication" short:"u" env:"USERNAME"`
	Password           string        `help:"Password to use for authentication" short:"p" env:"PASSWORD"`
	Port               int           `help:"Default port to use when connecting to cluster" default:"9042" short:"r" env:"PORT"`
	ProtocolVersion    string        `help:"Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"n" env:"PROTOCOL_VERSION"`
	MaxProtocolVersion string        `help:"Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"m" env:"MAX_PROTOCOL_VERSION"`
	Bind               string        `help:"Address to use to bind server" short:"a" default:":9042" env:"BIND"`
	Debug              bool          `help:"Show debug logging" default:"false" env:"DEBUG"`
	HealthCheck        bool          `help:"Enable liveness and readiness checks" default:"false" env:"HEALTH_CHECK"`
	HttpBind           string        `help:"Address to use to bind HTTP server used for health checks" default:":8000" env:"HTTP_BIND"`
	HeartbeatInterval  time.Duration `help:"Interval between performing heartbeats to the cluster" default:"30s" env:"HEARTBEAT_INTERVAL"`
	IdleTimeout        time.Duration `help:"Duration between successful heartbeats before a connection to the cluster is considered unresponsive and closed" default:"60s" env:"IDLE_TIMEOUT"`
	ReadinessTimeout   time.Duration `help:"Duration the proxy is unable to connect to the backend cluster before it is considered not ready" default:"30s" env:"READINESS_TIMEOUT"`
	NumConns           int           `help:"Number of connection to create to each node of the backend cluster" default:"1" env:"NUM_CONNS"`
}

// Run starts the proxy command. 'args' shouldn't include the executable (i.e. os.Args[1:]). It returns the exit code
// for the proxy.
func Run(ctx context.Context, args []string) int {
	var err error

	parser, err := kong.New(&cli)
	if err != nil {
		panic(err)
	}

	var cliCtx *kong.Context
	if cliCtx, err = parser.Parse(args); err != nil {
		parser.Errorf("error parsing cli: %v", err)
		return 1
	}

	var resolver proxycore.EndpointResolver
	if len(cli.AstraBundle) > 0 {
		if bundle, err := astra.LoadBundleZipFromPath(cli.AstraBundle); err != nil {
			cliCtx.Errorf("unable to open bundle %s from file: %v", cli.AstraBundle, err)
			return 1
		} else {
			resolver = astra.NewResolver(bundle)
		}
	} else if len(cli.AstraToken) > 0 {
		if len(cli.AstraDatabaseID) == 0 {
			cliCtx.Fatalf("database ID is required when using a token")
		}
		bundle, err := astra.LoadBundleZipFromURL(cli.AstraAPIURL, cli.AstraDatabaseID, cli.AstraToken, 10*time.Second)
		if err != nil {
			cliCtx.Fatalf("unable to load bundle %s from astra: %v", cli.AstraBundle, err)
		}
		resolver = astra.NewResolver(bundle)
		cli.Username = "token"
		cli.Password = cli.AstraToken
	} else if len(cli.ContactPoints) > 0 {
		resolver = proxycore.NewResolverWithDefaultPort(cli.ContactPoints, cli.Port)
	} else {
		cliCtx.Errorf("must provide either bundle path, token, or contact points")
		return 1
	}

	if cli.HeartbeatInterval >= cli.IdleTimeout {
		cliCtx.Errorf("idle-timeout must be greater than heartbeat-interval (heartbeat interval: %s, idle timeout: %s)",
			cli.HeartbeatInterval, cli.IdleTimeout)
		return 1
	}

	if cli.NumConns < 1 {
		cliCtx.Errorf("invalid number of connections, must be greater than 0 (provided: %d)", cli.NumConns)
		return 1
	}

	var ok bool
	var version primitive.ProtocolVersion
	if version, ok = parseProtocolVersion(cli.ProtocolVersion); !ok {
		cliCtx.Errorf("unsupported protocol version: %s", cli.ProtocolVersion)
		return 1
	}

	var maxVersion primitive.ProtocolVersion
	if maxVersion, ok = parseProtocolVersion(cli.MaxProtocolVersion); !ok {
		cliCtx.Errorf("unsupported max protocol version: %s", cli.ProtocolVersion)
		return 1
	}

	if version > maxVersion {
		cliCtx.Errorf("default protocol version is greater than max protocol version")
		return 1
	}

	var logger *zap.Logger
	if cli.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		cliCtx.Errorf("unable to create logger")
		return 1
	}

	var auth proxycore.Authenticator

	if len(cli.Username) > 0 || len(cli.Password) > 0 {
		auth = proxycore.NewPasswordAuth(cli.Username, cli.Password)
	}

	p := NewProxy(ctx, Config{
		Version:           version,
		MaxVersion:        maxVersion,
		Resolver:          resolver,
		ReconnectPolicy:   proxycore.NewReconnectPolicy(),
		NumConns:          cli.NumConns,
		Auth:              auth,
		Logger:            logger,
		HeartBeatInterval: cli.HeartbeatInterval,
		IdleTimeout:       cli.IdleTimeout,
	})

	cli.Bind = maybeAddPort(cli.Bind, "9042")
	cli.HttpBind = maybeAddPort(cli.HttpBind, "8000")

	maybeAddHealthCheck(p)

	err = listenAndServe(p, ctx, logger)
	if err != nil {
		cliCtx.Errorf("%v", err)
		return 1
	}

	return 0
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

// maybeAddHealthCheck checks the cli flag and adds handlers for health checks if required.
func maybeAddHealthCheck(p *Proxy) {
	if cli.HealthCheck {
		http.HandleFunc(livenessPath, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ok"))
		})
		http.HandleFunc(readinessPath, func(writer http.ResponseWriter, request *http.Request) {
			header := writer.Header()
			header.Set("Content-Type", "application/json")

			outageDuration := p.OutageDuration()
			response, err := json.Marshal(struct {
				OutageDuration string
			}{outageDuration.String()})
			if err != nil {
				http.Error(writer, fmt.Sprintf("failed to marshal json response: %v", err), http.StatusInternalServerError)
				return
			}

			if outageDuration < cli.ReadinessTimeout {
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(response)
			} else {
				writer.WriteHeader(http.StatusServiceUnavailable)
				_, _ = writer.Write(response)
			}
		})
	}
}

// maybeAddPort adds the default port to an IP; otherwise, it returns the original address.
func maybeAddPort(addr string, defaultPort string) string {
	if net.ParseIP(addr) != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// listenAndServe correctly handles serving both the proxy and an HTTP server simultaneously.
func listenAndServe(p *Proxy, ctx context.Context, logger *zap.Logger) (err error) {
	var wg sync.WaitGroup

	ch := make(chan error)
	server := http.Server{Addr: cli.HttpBind}

	numServers := 1 // Without the HTTP server

	// Listen is called first to set up the listening server connection and establish initial client connections to the
	// backend cluster so that when the readiness check is hit the proxy is actually ready.
	err = p.Listen(cli.Bind)
	if err != nil {
		return err
	}

	var listener *net.TCPListener

	if cli.HealthCheck {
		numServers++ // Add the HTTP server

		tcpAddr, err := net.ResolveTCPAddr("tcp", cli.HttpBind)
		if err != nil {
			return err
		}
		listener, err = net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			return err
		}

		logger.Info("health checks are listening",
			zap.String("livenessURL", cli.HttpBind+livenessPath),
			zap.String("readinessURL", cli.HttpBind+readinessPath))
	}

	wg.Add(numServers)

	go func() {
		select {
		case <-ctx.Done():
			logger.Debug("proxy interrupted/killed")
			_ = server.Close()
			_ = p.Shutdown()
		}
	}()

	go func() {
		defer wg.Done()
		err = p.Serve()
		if err != nil {
			ch <- err
		}
	}()

	if cli.HealthCheck {
		go func() {
			defer wg.Done()
			err = server.Serve(listener)
			if err != nil {
				ch <- err
			}
		}()

	}

	for err = range ch {
		if err != nil {
			return nil
		}
	}

	return err
}
