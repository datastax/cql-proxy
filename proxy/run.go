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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/datastax/cql-proxy/astra"
	"github.com/datastax/cql-proxy/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const livenessPath = "/liveness"
const readinessPath = "/readiness"

var config struct {
	AstraBundle        string        `yaml:"astra-bundle" help:"Path to secure connect bundle for an Astra database. Requires '--username' and '--password'. Ignored if using the token or contact points option." short:"b" env:"ASTRA_BUNDLE"`
	AstraToken         string        `yaml:"astra-token" help:"Token used to authenticate to an Astra database. Requires '--astra-database-id'. Ignored if using the bundle path or contact points option." short:"t" env:"ASTRA_TOKEN"`
	AstraDatabaseID    string        `yaml:"astra-database-id" help:"Database ID of the Astra database. Requires '--astra-token'" short:"i" env:"ASTRA_DATABASE_ID"`
	AstraApiURL        string        `yaml:"astra-api-url" help:"URL for the Astra API" default:"https://api.astra.datastax.com" env:"ASTRA_API_URL"`
	ContactPoints      []string      `yaml:"contact-points" help:"Contact points for cluster. Ignored if using the bundle path or token option." short:"c" env:"CONTACT_POINTS"`
	Username           string        `yaml:"username" help:"Username to use for authentication" short:"u" env:"USERNAME"`
	Password           string        `yaml:"password" help:"Password to use for authentication" short:"p" env:"PASSWORD"`
	Port               int           `yaml:"port" help:"Default port to use when connecting to cluster" default:"9042" short:"r" env:"PORT"`
	ProtocolVersion    string        `yaml:"protocol-version" help:"Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"n" env:"PROTOCOL_VERSION"`
	MaxProtocolVersion string        `yaml:"max-protocol-version" help:"Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"m" env:"MAX_PROTOCOL_VERSION"`
	Bind               string        `yaml:"bind" help:"Address to use to bind server" short:"a" default:":9042" env:"BIND"`
	Config             *os.File      `yaml:"-" help:"YAML configuration file" short:"f" env:"CONFIG_FILE"` // Not available in the configuration file
	Debug              bool          `yaml:"debug" help:"Show debug logging" default:"false" env:"DEBUG"`
	HealthCheck        bool          `yaml:"health-check" help:"Enable liveness and readiness checks" default:"false" env:"HEALTH_CHECK"`
	HttpBind           string        `yaml:"http-bind" help:"Address to use to bind HTTP server used for health checks" default:":8000" env:"HTTP_BIND"`
	HeartbeatInterval  time.Duration `yaml:"heartbeat-interval" help:"Interval between performing heartbeats to the cluster" default:"30s" env:"HEARTBEAT_INTERVAL"`
	IdleTimeout        time.Duration `yaml:"idle-timeout" help:"Duration between successful heartbeats before a connection to the cluster is considered unresponsive and closed" default:"60s" env:"IDLE_TIMEOUT"`
	ReadinessTimeout   time.Duration `yaml:"readiness-timeout" help:"Duration the proxy is unable to connect to the backend cluster before it is considered not ready" default:"30s" env:"READINESS_TIMEOUT"`
	NumConns           int           `yaml:"num-conns" help:"Number of connection to create to each node of the backend cluster" default:"1" env:"NUM_CONNS"`
	RpcAddress         string        `yaml:"rpc-address" help:"Address to advertise in the 'system.local' table for 'rpc_address'. It must be set if configuring peer proxies" env:"RPC_ADDRESS"`
	DataCenter         string        `yaml:"data-center" help:"Data center to use in system tables" env:"DATA_CENTER"`
	Tokens             []string      `yaml:"tokens" help:"Tokens to use in the system tables. It's not recommended" env:"TOKENS"`
	Peers              []PeerConfig  `yaml:"peers" kong:"-"` // Not available as a CLI flag
}

// Run starts the proxy command. 'args' shouldn't include the executable (i.e. os.Args[1:]). It returns the exit code
// for the proxy.
func Run(ctx context.Context, args []string) int {
	var err error

	parser, err := kong.New(&config)
	if err != nil {
		panic(err)
	}

	var cliCtx *kong.Context
	if cliCtx, err = parser.Parse(args); err != nil {
		parser.Errorf("error parsing flags: %v", err)
		return 1
	}

	if config.Config != nil {
		bytes, err := ioutil.ReadAll(config.Config)
		if err != nil {
			cliCtx.Errorf("unable to read contents of configuration file '%s': %v", config.Config.Name(), err)
			return 1
		}
		err = yaml.Unmarshal(bytes, &config)
		if err != nil {
			cliCtx.Errorf("invalid YAML in configuration file '%s': %v", config.Config.Name(), err)
		}
	}

	var resolver proxycore.EndpointResolver
	if len(config.AstraBundle) > 0 {
		if bundle, err := astra.LoadBundleZipFromPath(config.AstraBundle); err != nil {
			cliCtx.Errorf("unable to open bundle %s from file: %v", config.AstraBundle, err)
			return 1
		} else {
			resolver = astra.NewResolver(bundle)
		}
	} else if len(config.AstraToken) > 0 {
		if len(config.AstraDatabaseID) == 0 {
			cliCtx.Fatalf("database ID is required when using a token")
		}
		bundle, err := astra.LoadBundleZipFromURL(config.AstraApiURL, config.AstraDatabaseID, config.AstraToken, 10*time.Second)
		if err != nil {
			cliCtx.Fatalf("unable to load bundle %s from astra: %v", config.AstraBundle, err)
		}
		resolver = astra.NewResolver(bundle)
		config.Username = "token"
		config.Password = config.AstraToken
	} else if len(config.ContactPoints) > 0 {
		resolver = proxycore.NewResolverWithDefaultPort(config.ContactPoints, config.Port)
	} else {
		cliCtx.Errorf("must provide either bundle path, token, or contact points")
		return 1
	}

	if config.HeartbeatInterval >= config.IdleTimeout {
		cliCtx.Errorf("idle-timeout must be greater than heartbeat-interval (heartbeat interval: %s, idle timeout: %s)",
			config.HeartbeatInterval, config.IdleTimeout)
		return 1
	}

	if config.NumConns < 1 {
		cliCtx.Errorf("invalid number of connections, must be greater than 0 (provided: %d)", config.NumConns)
		return 1
	}

	var ok bool
	var version primitive.ProtocolVersion
	if version, ok = parseProtocolVersion(config.ProtocolVersion); !ok {
		cliCtx.Errorf("unsupported protocol version: %s", config.ProtocolVersion)
		return 1
	}

	var maxVersion primitive.ProtocolVersion
	if maxVersion, ok = parseProtocolVersion(config.MaxProtocolVersion); !ok {
		cliCtx.Errorf("unsupported max protocol version: %s", config.ProtocolVersion)
		return 1
	}

	if version > maxVersion {
		cliCtx.Errorf("default protocol version is greater than max protocol version")
		return 1
	}

	var logger *zap.Logger
	if config.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		cliCtx.Errorf("unable to create logger")
		return 1
	}

	var auth proxycore.Authenticator

	if len(config.Username) > 0 || len(config.Password) > 0 {
		auth = proxycore.NewPasswordAuth(config.Username, config.Password)
	}

	p := NewProxy(ctx, Config{
		Version:           version,
		MaxVersion:        maxVersion,
		Resolver:          resolver,
		ReconnectPolicy:   proxycore.NewReconnectPolicy(),
		NumConns:          config.NumConns,
		Auth:              auth,
		Logger:            logger,
		HeartBeatInterval: config.HeartbeatInterval,
		IdleTimeout:       config.IdleTimeout,
		RPCAddr:           config.RpcAddress,
		DC:                config.DataCenter,
		Tokens:            config.Tokens,
		Peers:             config.Peers,
	})

	config.Bind = maybeAddPort(config.Bind, "9042")
	config.HttpBind = maybeAddPort(config.HttpBind, "8000")

	var mux http.ServeMux
	maybeAddHealthCheck(p, &mux)

	err = listenAndServe(p, &mux, ctx, logger)
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

// maybeAddHealthCheck checks the config and adds handlers for health checks if required.
func maybeAddHealthCheck(p *Proxy, mux *http.ServeMux) {
	if config.HealthCheck {
		mux.HandleFunc(livenessPath, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ok"))
		})
		mux.HandleFunc(readinessPath, func(writer http.ResponseWriter, request *http.Request) {
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

			if outageDuration < config.ReadinessTimeout {
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
func listenAndServe(p *Proxy, mux *http.ServeMux, ctx context.Context, logger *zap.Logger) (err error) {
	var wg sync.WaitGroup

	ch := make(chan error)
	server := http.Server{Addr: config.HttpBind, Handler: mux}

	numServers := 1 // Without the HTTP server

	// Listen is called first to set up the listening server connection and establish initial client connections to the
	// backend cluster so that when the readiness check is hit the proxy is actually ready.
	err = p.Listen(config.Bind)
	if err != nil {
		return err
	}

	var listener *net.TCPListener

	if config.HealthCheck {
		numServers++ // Add the HTTP server

		tcpAddr, err := net.ResolveTCPAddr("tcp", config.HttpBind)
		if err != nil {
			return err
		}
		listener, err = net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			return err
		}

		logger.Info("health checks are listening",
			zap.String("livenessURL", config.HttpBind+livenessPath),
			zap.String("readinessURL", config.HttpBind+readinessPath))
	}

	wg.Add(numServers)

	go func() {
		wg.Wait()
		close(ch)
	}()

	go func() {
		select {
		case <-ctx.Done():
			logger.Debug("proxy interrupted/killed")
			_ = server.Close()
			_ = p.Close()
		}
	}()

	go func() {
		defer wg.Done()
		err := p.Serve()
		if err != nil && err != ErrProxyClosed {
			ch <- err
		}
	}()

	if config.HealthCheck {
		go func() {
			defer wg.Done()
			err := server.Serve(listener)
			if err != nil && err != http.ErrServerClosed {
				ch <- err
			}
		}()

	}

	for err = range ch {
		if err != nil {
			return err
		}
	}

	return err
}
