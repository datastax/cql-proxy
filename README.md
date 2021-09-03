# cql-proxy

[![GitHub Action](https://github.com/datastax/cql-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/datastax/cql-proxy/actions/workflows/test.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/datastax/cql-proxy)](https://goreportcard.com/report/github.com/datastax/cql-proxy)

A CQL proxy/sidecar. It listens on a local address and securely forwards your application's CQL traffic.

**Note**: CQL proxy in its current state works well, but it is still under development. That means
that things might break or change. Please give it a try and let us know what you think!

![cql-proxy](cql-proxy.png)

## Getting Started

```sh
go build
```

Run against an [Astra cluster][astra]:

```sh
./cql-proxy --bundle <your-secure-connect-zip> --username <astra-client-id> --password <astra-client-secret>
```

or using Docker as

```sh
docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042 \
  --rm datastax/cql-proxy:v0.0.2 \
  --bundle /tmp/scb.zip --username <astra-client-id> --password <astra-client-secret>
```

`<astra-client-id>` and `<astra-client-secret>` can be generated using these [instructions].

Run against a Apache Cassandra cluster:

```sh
./cql-proxy --contact-points <cluster node IPs or DNS names>
```

or using Docker as

```sh
docker run -p 9042:9042 \
  --rm datastax/cql-proxy:v0.0.2 \
  --contact-points <cluster node IPs or DNS names>
```

## Configuration

To pass configuration to the cql-proxy both command line flags and environment variables can be used. Below are examples of
the same command using both methods

Flags

```sh
docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042 \
  --rm datastax/cql-proxy:v0.0.2 \
  --bundle /tmp/scb.zip --username <astra-client-id> --password <astra-client-secret>
```

Environment Variables

```sh
docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042  \
  --rm datastax/cql-proxy:v0.0.2 \
  -e BUNDLE=/tmp/scb.zip -e USERNAME=<astra-client-id> -e PASSWORD=<astra-client-secret>
```

To see what options are available the `-h` flag will display a help message listing all flags and their corresponding descriptions
and environment variables

```sh
$ ./cql-proxy -h
Usage: cql-proxy

Flags:
  -h, --help               Show context-sensitive help.
  -b, --bundle=STRING      Path to secure connect bundle ($BUNDLE)
  -u, --username=STRING    Username to use for authentication ($USERNAME)
  -p, --password=STRING    Password to use for authentication ($PASSWORD)
  -c, --contact-points=CONTACT-POINTS,...
                           Contact points for cluster. Ignored if using the bundle path
                           option ($CONTACT_POINTS).
  -a, --bind=STRING        Address to use to bind serve ($BIND)
      --debug              Show debug logging ($DEBUG)
      --profiling          Enable profiling ($PROFILING)
```

[astra]: https://astra.datastax.com/
[instructions]: https://docs.datastax.com/en/astra/docs/manage-application-tokens.html
