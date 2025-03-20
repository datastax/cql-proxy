module github.com/datastax/cql-proxy

go 1.23.0

toolchain go1.23.6

require (
	github.com/alecthomas/kong v0.2.17
	github.com/datastax/go-cassandra-native-protocol v0.0.0-20211124104234-f6aea54fa801
	github.com/hashicorp/golang-lru v0.5.4
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.8.0
	go.uber.org/zap v1.17.0
)

require (
	github.com/datastax/astra-client-go/v2 v2.2.9 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
