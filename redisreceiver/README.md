# Redis Receiver

The Redis receiver is designed to retrieve Redis INFO data from a single Redis
instance, build metrics from that data, and send them to the next consumer at a
configurable interval.

Supported pipeline types: metrics

> :construction: This receiver is in beta and configuration fields are subject to change.

## Details

The Redis INFO command returns information and statistics about a Redis
server (see [https://redis.io/commands/info](https://redis.io/commands/info) for
details). The Redis receiver extracts values from the result and converts them to open
telemetry metrics. Details about the metrics produced by the Redis receiver
can be found by browsing [metric_functions.go](metric_functions.go).

For example, one of the fields returned by the Redis INFO command is
`used_cpu_sys` which indicates the system CPU consumed by the Redis server,
expressed in seconds, since the start of the Redis instance.

The Redis receiver turns this data into a gauge...

```go
func usedCPUSys() *redisMetric {
	return &redisMetric{
		key:    "used_cpu_sys",
		name:   "redis.cpu.time",
		units:  "s",
		mdType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
		labels: map[string]string{"state": "sys"},
	}
}
```

with a metric name of `redis.cpu.time` and a units value of `s` (seconds).

## Configuration

> :information_source: This receiver is in beta and configuration fields are subject to change.

The following settings are required:

- `endpoint` (no default): The hostname and port of the Redis instance,
separated by a colon.

The following settings are optional:

- `collection_interval` (default = `10s`): This receiver runs on an interval.
Each time it runs, it queries Redis, creates metrics, and sends them to the
next consumer. The `collection_interval` configuration option tells this
receiver the duration between runs. This value must be a string readable by
Golang's `ParseDuration` function (example: `1h30m`). Valid time units are
`ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `password` (no default): The password used to access the Redis instance;
must match the password specified in the `requirepass` server configuration
option.
- `transport` (default = `tcp`) Defines the network to use for connecting to the server. Valid Values are `tcp` or `Unix`
- `tls`:
  - `insecure` (default = true): whether to disable client transport security for the exporter's connection.
  - `ca_file`: path to the CA cert. For a client this verifies the server certificate. Should only be used if `insecure` is set to false.
  - `cert_file`: path to the TLS cert to use for TLS required connections. Should only be used if `insecure` is set to false.
  - `key_file`: path to the TLS key to use for TLS required connections. Should only be used if `insecure` is set to false.

Example:

```yaml
receivers:
  redis:
    endpoint: "localhost:6379"
    collection_interval: 10s
    password: $REDIS_PASSWORD
```

> :information_source: As with all Open Telemetry configuration values, a
reference to an environment variable is supported. For example, to pick up
the value of an environment variable `REDIS_PASSWORD`, you could use a
configuration like the following:

```yaml
receivers:
  redis:
    endpoint: "localhost:6379"
    collection_interval: 10s
    password: $REDIS_PASSWORD
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
