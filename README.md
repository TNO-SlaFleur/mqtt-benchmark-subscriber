MQTT benchmarking tool
=========

FORKED FROM https://github.com/krylovsk/mqtt-benchmark

A simple MQTT (broker) benchmarking tool that is supposed to be used with https://github.com/TNO-SlaFleur/mqtt-benchmark-publisher.
The goal is to measure inflight latency using published/sending & subscription/received timestamps.

Installation:

```sh
go install github.com/TNO-SlaFleur/mqtt-benchmark-subscriber@main
```

The tool supports multiple concurrent clients, configurable message size, etc:

```sh
$ ./mqtt-benchmark-subscriber --help
Usage of ./mqtt-benchmark-subscriber:
  -broker string
    	MQTT broker endpoint as scheme://host:port (default "tcp://localhost:1883")
  -client-cert string
    	Path to client certificate in PEM format
  -client-key string
    	Path to private clientKey in PEM format
  -client-prefix string
    	MQTT client id prefix (suffixed with '-<client-num>' (default "mqtt-benchmark")
  -clients int
    	Number of clients to start (default 10)
  -count int
    	Number of messages to receive per client (default 100)
  -format string
    	Output format: text|json (default "text")
  -password string
    	MQTT client password (empty if auth disabled)
  -qos int
    	QoS for published messages (default 1)
  -quiet
    	Suppress logs while running
  -topic string
    	MQTT topic for outgoing messages (default "/test")
  -username string
    	MQTT client username (empty if auth disabled)
```

> NOTE: if `count=1` or `clients=1`, the sample standard deviation will be returned as `0` (convention due to the [lack of NaN support in JSON](https://tools.ietf.org/html/rfc4627#section-2.4))

Two output formats supported: human-readable plain text and JSON.

Example use and output:

```sh
> mqtt-benchmark --broker tcp://broker.local:1883 --count 100 --size 100 --clients 100 --qos 2 --format text
....
TBD
```

With payload specified:

```sh
> mqtt-benchmark --broker tcp://broker.local:1883 --count 100 --clients 10 --qos 1 --topic house/bedroom/temperature --payload {\"temperature\":20,\"timeStamp\":1597314150}
....

TBD
```

Similarly, in JSON:

```json
> mqtt-benchmark --broker tcp://broker.local:1883 --count 100 --size 100 --clients 100 --qos 2 --format json --quiet
TBD
```
