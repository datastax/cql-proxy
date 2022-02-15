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
  -h, --help                                 Show context-sensitive help.
  -b, --astra-bundle=STRING                  Path to secure connect bundle for an Astra database. Requires '--username' and '--password'. Ignored if using the token or contact points option
                                             ($ASTRA_BUNDLE).
  -t, --astra-token=STRING                   Token used to authenticate to an Astra database. Requires '--astra-database-id'. Ignored if using the bundle path or contact points option
                                             ($ASTRA_TOKEN).
  -i, --astra-database-id=STRING             Database ID of the Astra database. Requires '--astra-token' ($ASTRA_DATABASE_ID)
  -c, --contact-points=CONTACT-POINTS,...    Contact points for cluster. Ignored if using the bundle path or token option ($CONTACT_POINTS).
  -u, --username=STRING                      Username to use for authentication ($USERNAME)
  -p, --password=STRING                      Password to use for authentication ($PASSWORD)
  -r, --port=9042                            Default port to use when connecting to cluster ($PORT)
  -n, --protocol-version="v4"                Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2) ($PROTOCOL_VERSION)
  -m, --max-protocol-version="v4"            Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2) ($MAX_PROTOCOL_VERSION)
  -a, --bind=":9042"                         Address to use to bind server ($BIND)
      --debug                                Show debug logging ($DEBUG)
      --health-check                         Enable liveness and readiness checks ($HEALTH_CHECK)
      --http-bind=":8000"                    Address to use to bind HTTP server used for health checks ($HTTP_BIND)
      --heartbeat-interval=30s               Interval between performing heartbeats to the cluster ($HEARTBEAT_INTERVAL)
      --idle-timeout=60s                     Duration between successful heartbeats before a connection to the cluster is considered unresponsive and closed ($IDLE_TIMEOUT)
      --readiness-timeout=30s                Duration the proxy is unable to connect to the backend cluster before it is considered not ready ($READINESS_TIMEOUT)
      --num-conns=1                          Number of connection to create to each node of the backend cluster ($NUM_CONNS)
```

To pass configuration to `cql-proxy`, either command-line flags or environment variables can be used. Using the `docker` method as an example, the following samples show how the token and database ID are defined with each method.
### Using flags

```sh
docker run -p 9042:9042 \
  --rm datastax/cql-proxy:v0.1.0 \
  --astra-token <astra-token> --astra-database-id <astra-datbase-id>
```

### Using environment variables

```sh
docker run -p 9042:9042  \
  --rm datastax/cql-proxy:v0.1.0 \
  -e ASTRA_TOKEN=<astra-token> -e ASTRA_DATABASE_ID=<astra-datbase-id>
```
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
      ./cql-proxy --bundle <your-secure-connect-zip> \
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
        datastax/cql-proxy:v0.1.0 \
        --astra-token <astra-token> --astra-database-id <astra-database-id>
      ```

      The `<astra-token>` can be generated using these [instructions]. The proxy also supports using the [Astra Secure Connect Bundle][bundle], but it requires mounting the bundle to a volume in the container:

      ```sh
      docker run -v <your-secure-connect-bundle.zip>:/tmp/scb.zip -p 9042:9042 \
      --rm datastax/cql-proxy:v0.1.0 \
      --astra-bundle /tmp/scb.zip --username <astra-client-id> --password <astra-client-secret>
      ```
   - [Apache Cassandra][cassandra] cluster:

      ```sh
      docker run -p 9042:9042 \
        datastax/cql-proxy:v0.1.0 \
        --contact-points <cluster node IPs or DNS names> [--username <username>] [--password <password>]
      ```
  If you wish to have the docker image removed after you are done with it, add `--rm` before the image name `datastax/cql-proxy:v0.1.0 `.

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

[astra]: https://astra.datastax.com/
[drivers]: https://docs.datastax.com/en/driver-matrix/doc/driver_matrix/common/driverMatrix.html
[gocql]: https://github.com/gocql/gocql
[rust-driver]: https://github.com/scylladb/scylla-rust-driver
[driver-guide]: https://docs.datastax.com/en/astra/docs/connecting-to-astra-databases-using-datastax-drivers.html
[cassandra]: https://cassandra.apache.org/
[dse]: https://www.datastax.com/products/datastax-enterprise
[instructions]: https://docs.datastax.com/en/astra/docs/manage-application-tokens.html
[bundle]: https://docs.datastax.com/en/astra/docs/obtaining-database-credentials.html#_getting_your_secure_connect_bundle


