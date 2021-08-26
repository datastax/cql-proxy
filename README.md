# cql-proxy

[![GitHub Action](https://github.com/datastax/cql-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/datastax/cql-proxy/actions/workflows/test.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/datastax/cql-proxy)](https://goreportcard.com/report/github.com/datastax/cql-proxy)


A CQL proxy/sidecar. It listens on a local address and securely forwards your application's CQL traffic.

## Getting Started

```sh
go build
```

Run against an [Astra cluster][astra]:

```sh
./cql-proxy --bundle <your-secure-connect-zip> --username token --password <your-astra-token>
```

or using Docker as

```sh
docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042 --rm datastax/cql-proxy:v0.0.1 \
--bundle /tmp/scb.zip --username token --password <your-astra-token>
```

Note: Use the literal `token` as the username.

Run against a Apache Cassandra cluster:

```sh
./cql-proxy --contact-points <cluster node IPs or DNS names>
```

or using Docker as

```sh
docker run -p 9042:9042 --rm datastax/cql-proxy:v0.0.1 --contact-points <cluster node IPs or DNS names>
```

[astra]: https://astra.datastax.com/
