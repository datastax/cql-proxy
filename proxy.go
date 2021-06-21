package cqlproxy

// TODO:

// # Frame parsing
// * Learn: github.com/datastax/go-cassandra-native-protocol
// # Result set construction and parsing

// # Backend
// * Proxy-to-server CQL connection
//   - Read/Write requests
//   - Retry when down until removed (exponential backoff)
//   - Heartbeat
//   - Stream management
// * Control connection
//   - Query system.local/system.peers
//   - ADD/REMOVE/UP and schema events (channels)
//   - Contact point resolver
// * Sessions
//   - Pool connections and connection lifecycle
//   - Simple load-balancing (round-robin to start), concurrency!
//   - Keyspace state (USE <keyspace> problem, intercept and create new sessions)
// * Cloud
//   - Metadata service contact point resolver
//   - Endpoint type (with cluster DNS and SNI name, TLS config?)
//   - Make sure DNS round-robins A-records

// # Frontend
// * Client-to-proxy CQL connection, worker pool, httpfast
// * Fast CQL parser (limited recursive descent parser?)
//   - Intercept `system.local` and `system.peers` queries and USE <keyspace>
//   - Example: https://github.com/mpenick/cql-proxy/blob/main/src/parse.h
// * Pass through other query types, raw
