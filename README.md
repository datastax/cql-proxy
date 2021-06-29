# cql-proxy

A CQL proxy/sidecar. It listens on a local address and securely forwards your application's CQL traffic.

## Getting Started:

```
go build
```

Run against an [Astra cluster][astra]:

```
./cql-proxy --bundle <your-secure-connect-zip> --username token --password <your-astra-token>
```

Note: Use the literal `token` as the username.

Run against a Apache Cassandra cluster:

```
./cql-proxy --contact-points <cluster node IPs or DNS names>
```

[astra]: https://astra.datastax.com/