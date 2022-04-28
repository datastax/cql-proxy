# cql-proxy

[![GitHub Action](https://github.com/datastax/cql-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/datastax/cql-proxy/actions/workflows/test.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/datastax/cql-proxy)](https://goreportcard.com/report/github.com/datastax/cql-proxy)

## Table of Contents

- [What is the cql-proxy?](https://github.com/datastax/cql-proxy#what-is-cqlproxy)
- [When to use cql-proxy](https://github.com/datastax/cql-proxy#when-to-use-cql-proxy)
- [Configuration](https://github.com/datastax/cql-proxy#configuration)
- [Getting started](https://github.com/datastax/cql-proxy#getting-started)
  - [Locally build and run](https://github.com/datastax/cql-proxy#locally-build-and-run)
  - [Run a `cql-proxy` docker image](https://github.com/datastax/cql-proxy#run-a-cql-proxy-docker-image)
  - [Use Kubernetes](https://github.com/datastax/cql-proxy#use-kubernetes)
- [Known issues](https://github.com/datastax/cql-proxy#known-issues)
    - [Token-aware load balancing](https://github.com/datastax/cql-proxy#token-aware-load-balancing)


## What is `cql-proxy`?

![cql-proxy](cql-proxy.png)

`cql-proxy` is designed to forward your application's CQL traffic to an appropriate database service. It listens on a local address and securely forwards that traffic.

## When to use `cql-proxy`

The `cql-proxy` sidecar enables unsupported CQL drivers to work with [DataStax Astra][astra]. These drivers include both legacy DataStax [drivers] and community-maintained CQL drivers, such as the [gocql] driver and the [rust-driver].

`cql-proxy` also enables applications that are currently using [Apache Cassandra][cassandra] or [DataStax Enterprise (DSE)][dse] to use Astra without requiring any code changes.  Your application just needs to be configured to use the proxy.

If you're building a new application using DataStax [drivers], `cql-proxy` is not required, as the drivers can communicate directly with Astra. DataStax drivers have excellent support for Astra out-of-the-box, and are well-documented in the [driver-guide] guide. 

## Configuration

Use the `-h` or `--help` flag to display a listing all flags and their corresponding descriptions and environment variables (shown below as items starting with `$`):

```sh
$ ./cql-proxy -h
Usage: cql-proxy

Flags:
  -h, --help                                              Show context-sensitive help.
  -b, --astra-bundle=STRING                               Path to secure connect bundle for an Astra database. Requires '--username' and '--password'. Ignored if using the token or contact points
                                                          option ($ASTRA_BUNDLE).
  -t, --astra-token=STRING                                Token used to authenticate to an Astra database. Requires '--astra-database-id'. Ignored if using the bundle path or contact points option
                                                          ($ASTRA_TOKEN).
  -i, --astra-database-id=STRING                          Database ID of the Astra database. Requires '--astra-token' ($ASTRA_DATABASE_ID)
      --astra-api-url="https://api.astra.datastax.com"    URL for the Astra API ($ASTRA_API_URL)
      --astra-host=STRING                                 Overrides the Astra DB host. It's useful when using a cloud private endpoint service ($ASTRA_HOST)
      --astra-timeout=10s                                 Timeout for contacting Astra when retrieving the bundle and metadata ($ASTRA_TIMEOUT)
  -c, --contact-points=CONTACT-POINTS,...                 Contact points for cluster. Ignored if using the bundle path or token option ($CONTACT_POINTS).
  -u, --username=STRING                                   Username to use for authentication ($USERNAME)
  -p, --password=STRING                                   Password to use for authentication ($PASSWORD)
  -r, --port=9042                                         Default port to use when connecting to cluster ($PORT)
  -n, --protocol-version="v4"                             Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2) ($PROTOCOL_VERSION)
  -m, --max-protocol-version="v4"                         Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2) ($MAX_PROTOCOL_VERSION)
  -a, --bind=":9042"                                      Address to use to bind server ($BIND)
  -f, --config=CONFIG                                     YAML configuration file ($CONFIG_FILE)
      --debug                                             Show debug logging ($DEBUG)
      --health-check                                      Enable liveness and readiness checks ($HEALTH_CHECK)
      --http-bind=":8000"                                 Address to use to bind HTTP server used for health checks ($HTTP_BIND)
      --heartbeat-interval=30s                            Interval between performing heartbeats to the cluster ($HEARTBEAT_INTERVAL)
      --idle-timeout=60s                                  Duration between successful heartbeats before a connection to the cluster is considered unresponsive and closed ($IDLE_TIMEOUT)
      --readiness-timeout=30s                             Duration the proxy is unable to connect to the backend cluster before it is considered not ready ($READINESS_TIMEOUT)
      --idempotent-graph                                  If true it will treat all graph queries as idempotent by default and retry them automatically. It may be dangerous to retry some graph
                                                          queries -- use with caution ($IDEMPOTENT_GRAPH).
      --num-conns=1                                       Number of connection to create to each node of the backend cluster ($NUM_CONNS)
      --rpc-address=STRING                                Address to advertise in the 'system.local' table for 'rpc_address'. It must be set if configuring peer proxies ($RPC_ADDRESS)
      --data-center=STRING                                Data center to use in system tables ($DATA_CENTER)
      --tokens=TOKENS,...                                 Tokens to use in the system tables. It's not recommended ($TOKENS)
```

To pass configuration to `cql-proxy`, either command-line flags, environment variables, or a configuration file can be used. Using the `docker` method as an example, the following samples show how the token and database ID are defined with each method.
### Using flags

```sh
docker run -p 9042:9042 \
  --rm datastax/cql-proxy:v0.1.2 \
  --astra-token <astra-token> --astra-database-id <astra-datbase-id>
```

### Using environment variables

```sh
docker run -p 9042:9042  \
  --rm datastax/cql-proxy:v0.1.2 \
  -e ASTRA_TOKEN=<astra-token> -e ASTRA_DATABASE_ID=<astra-datbase-id>
```

### Using a configuration file

Proxy settings can also be passed using a configuration file with the `--config /path/to/proxy.yaml` flag. This can be mixed and matched with command-line flags and environment variables. Here are some example configuration files:

```yaml
contact-points:
  - 127.0.0.1
username: cassandra
password: cassandra
port: 9042
bind: 127.0.0.1:9042
# ...
```

or with a Astra token:

```yaml
astra-token: <astra-token>
astra-database-id: <astra-database-id>
bind: 127.0.0.1:9042
# ...
```

All configuration keys match their command-line flag counterpart, e.g. `--astra-bundle` is
`astra-bundle:`,  `--contact-points` is `contact-points:` etc.

#### Setting up peer proxies

Multi-region failover with DC-aware load balancing policy is the most useful case for a multiple proxy setup.

When configuring `peers:` it is required to set `--rpc-address` (or `rpc-address:` in the yaml) for each proxy and it must match is corresponding `peers:` entry. Also, `peers:` is only available in the configuration file and cannot be set using a command-line flag.

##### Multi-region setup

Here's an example of configuring multi-region failover with two proxies. A proxy is started for each region of the cluster connecting to it using that region's bundle. They all share a common configuration file that contains the full list of proxies.

*Note:* Only bundles are supported for multi-region setups.

```sh
cql-proxy --astra-bundle astra-region1-bundle.zip --username token --password <astra-token> \
  --bind 127.0.0.1:9042 --rpc-address 127.0.0.1 --data-center dc-1 --config proxy.yaml
```

```sh
cql-proxy ---astra-bundle astra-region2-bundle.zip --username token --password <astra-token> \
  --bind 127.0.0.2:9042 --rpc-address 127.0.0.2 --data-center dc-2 --config proxy.yaml
```

The peers settings are configured using a yaml file. It's a good idea to explicitly provide the `--data-center` flag, otherwise; these values are pulled from the backend cluster and would need to be pulled from the `system.local` and `system.peers` table to properly setup the peers `data-center:` values. Here's an example `proxy.yaml`:

```yaml
peers:
  - rpc-address: 127.0.0.1
    data-center: dc-1
  - rpc-address: 127.0.0.2
    data-center: dc-2
```

*Note:* It's okay for the `peers:` to contain entries for the current proxy itself because they'll just be omitted.

## Getting started

There are three methods for using `cql-proxy`:

- Locally build and run `cql-proxy`
- Run a docker image that has `cql-proxy` installed
- Use a Kubernetes container to run `cql-proxy`
### Locally build and run

1. Build `cql-proxy`.

    ```sh
    go build
    ```

2. Run with your desired database.

   - [DataStax Astra][astra] cluster:

      ```sh
      ./cql-proxy --astra-token <astra-token> --astra-database-id <astra-database-id>
      ```

      The `<astra-token>` can be generated using these [instructions]. The proxy also supports using the [Astra Secure Connect Bundle][bundle] along with a client ID and secret generated using these [instructions]:

      ```
      ./cql-proxy --astra-bundle <your-secure-connect-zip> \
      --username <astra-client-id> --password <astra-client-secret>
      ```

   - [Apache Cassandra][cassandra] cluster:

      ```sh
      ./cql-proxy --contact-points <cluster node IPs or DNS names> [--username <username>] [--password <password>]
      ```
### Run a `cql-proxy` docker image

1. Run with your desired database.

   - [DataStax Astra][astra] cluster:

      ```sh
      docker run -p 9042:9042 \
        datastax/cql-proxy:v0.1.2 \
        --astra-token <astra-token> --astra-database-id <astra-database-id>
      ```

      The `<astra-token>` can be generated using these [instructions]. The proxy also supports using the [Astra Secure Connect Bundle][bundle], but it requires mounting the bundle to a volume in the container:

      ```sh
      docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042 \
      --rm datastax/cql-proxy:v0.1.2 \
      --astra-bundle /tmp/scb.zip --username <astra-client-id> --password <astra-client-secret>
      ```
   - [Apache Cassandra][cassandra] cluster:

      ```sh
      docker run -p 9042:9042 \
        datastax/cql-proxy:v0.1.2 \
        --contact-points <cluster node IPs or DNS names> [--username <username>] [--password <password>]
      ```
  If you wish to have the docker image removed after you are done with it, add `--rm` before the image name `datastax/cql-proxy:v0.1.2`.

### Use Kubernetes

Using Kubernetes with `cql-proxy` requires a number of steps:

1. Generate a token following the Astra [instructions](https://docs.datastax.com/en/astra/docs/manage-application-tokens.html#_create_application_token). This step will display your Client ID, Client Secret, and Token; make sure you download the information for the next steps. Store the secure bundle in `/tmp/scb.zip` to match the example below.

2. Create `cql-proxy.yaml`. You'll need to add three sets of information: arguments, volume mounts, and volumes.

 - Argument: Modify the local bundle location, username and password, using the client ID and client secret obtained in the last step to the container argument.   

      ```
      command: ["./cql-proxy"]
      args: ["--astra-bundle=/tmp/scb.zip","--username=Client ID","--password=Client Secret"]
      ```

- Volume mounts: Modify `/tmp/` as a volume mount as required.

      volumeMounts:
        - name: my-cm-vol
        mountPath: /tmp/
 

- Volume: Modify the `configMap` filename as required. In this example, it is named `cql-proxy-configmap`. Use the same name for the `volumes` that you used for the `volumeMounts`. 

      volumes:
        - name: my-cm-vol
          configMap:
            name: cql-proxy-configmap        
    
3. Create a configmap. Use the same secure bundle that was specified in the `cql-proxy.yaml`.
      
      ```sh
      kubectl create configmap cql-proxy-configmap --from-file /tmp/scb.zip 
      ```

4. Check the configmap that was created. 

    ```sh
    kubectl describe configmap config
      
      Name:         config
      Namespace:    default
      Labels:       <none>
      Annotations:  <none>

      Data
      ====

      BinaryData
      ====
      scb.zip: 12311 bytes
    ```

5. Create a Kubernetes deployment with the YAML file you created:

     ```sh
     kubectl create -f cql-proxy.yaml
     ```

6. Check the logs:
    ```sh
    kubectl logs <deployment-name>
    ```
## Known issues

### Token-aware load balancing

Drivers that use token-aware load balancing may print a warning or may not work when using cql-proxy. Because cql-proxy abstracts the backend cluster as a single endpoint this doesn't always work well with token-aware drivers that expect there to be at least "replication factor" number of nodes in the cluster. Many drivers print a warning (which can be ignored) and fallback to something like round-robin, but other drivers might fail with an error. For the drivers that fail with an error it is required that they disable token-aware or configure the round-robin load balancing policy.

[astra]: https://astra.datastax.com/
[drivers]: https://docs.datastax.com/en/driver-matrix/doc/driver_matrix/common/driverMatrix.html
[gocql]: https://github.com/gocql/gocql
[rust-driver]: https://github.com/scylladb/scylla-rust-driver
[driver-guide]: https://docs.datastax.com/en/astra/docs/connecting-to-astra-databases-using-datastax-drivers.html
[cassandra]: https://cassandra.apache.org/
[dse]: https://www.datastax.com/products/datastax-enterprise
[instructions]: https://docs.datastax.com/en/astra/docs/manage-application-tokens.html
[bundle]: https://docs.datastax.com/en/astra/docs/obtaining-database-credentials.html#_getting_your_secure_connect_bundle
