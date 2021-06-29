package main

import (
	"context"
	"cql-proxy/astra"
	"cql-proxy/proxy"
	"cql-proxy/proxycore"
	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"net"
)

var cli struct {
	Bundle        string   `help:"Path to secure connect bundle" short:"b"`
	Username      string   `help:"Username to use for authentication" short:"u"`
	Password      string   `help:"Password to use for authentication" short:"p"`
	ContactPoints []string `help:"Contact points for cluster. Ignored if using the bundle path option." short:"c"`
	Bind          string   `help:"Address to use to bind serve" short:"a"`
	Debug         bool     `help:"Show debug logging"`
}

func main() {
	cliCtx := kong.Parse(&cli)

	var resolver proxycore.EndpointResolver

	if len(cli.Bundle) > 0 {
		bundle, err := astra.LoadBundleZip(cli.Bundle)
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
		auth = proxycore.NewDefaultAuth(cli.Username, cli.Password)
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

	err = p.ListenAndServe(bind)
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}
}
