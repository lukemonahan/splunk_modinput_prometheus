# Prometheus Metrics for Splunk

**WARNING**: This is a very early release and has undergone only limited testing. The version will be 1.0 when considered stable and complete.

Prometheus [prometheus.io](https://prometheus.io), a [Cloud Native Computing Foundation](https://cncf.io/) project, is a systems and service monitoring system. It collects metrics from configured targets at given intervals, evaluates rule expressions, displays the results, and can trigger alerts if some condition is observed to be true.

Splunk [splunk.com](https://www.splunk.com) is a platform for machine data analysis, providing real-time visibility, actionable insights and intelligence across all forms of machine data. Splunk Enterprise since version 7.0 includes the Metrics store for large-scale capture and analysis of time-series metrics alongside log data.

This Splunk add-on provides two modular inputs to create Splunk metrics from Prometheus data:

**[prometheus://]**

A scraping input which polls a Prometheus exporter and indexes all found metrics as Splunk metrics.

It is specifically also designed to be able to poll a Prometheus server's "federate" endpoint, so that Prometheus can be responsible for all the nitty-gritty (e.g. service discovery and label rewriting) and Splunk can gather a desired set of metrics from Prometheus at the desired resolution. In this way it acts much like Prometheus hierarchical federation.

It will also succesfully scrape a basic static Prometheus exporter, and in this use case does not require a Prometheus server at all.

**[prometheusrw://]**

A bridge so that the Prometheus remote-write feature can continuously deliver metrics to a Splunk Enterprise system for long-term storage, analysis and integration with other data sources in Splunk. It is structured as a Splunk app that provides a modular input implementing the remote-write bridge. When installed and enabled, this add-on will add a new listening port to your Splunk server which can be the target for multiple Prometheus servers remote write.

It has been designed to mimic the Splunk HTTP Event Collector from a configuration standpoint, however the endpoint is much simpler as it only support Prometheus remote-write. The HEC is not used as Prometheus remote-write requires Speedy compression and Protocol Buffer encoding, both of which do not work with the HEC.

## Architecture overview

### Static exporter
In this configuration, the modular input can poll any statically configured Prometheus exporter at a defined interval.
Pros:
 - Simple
 - Requires no Prometheus server
Cons:
 - Static configuration only -- no service discovery
 - HA of Splunk polling is difficult

<png>

### Federate server
With this configuration, the modular input is polling a Prometheus server that is federating the metrics from either exporters or other Prometheus servers.
Pros:
 - Allows Prometheus to handle service discovery and other low-level functions
 - High level of control of what Splunk gathers and when
 - Allows scenarios such as using Prometheus to gather high-resolution metrics, and gathering into Splunk at reduced frequency
Cons:
 - HA of Splunk polling is difficult
 - Could run into scalability issues if you want to gather large numbers of metrics from a single Prometheus server at a high rate

<png>

### Prometheus remote-write
With this configuration, Prometheus pushes metrics to Splunk with it's remote_write functionality.
Pros:
 - Most efficient way to ingest all, or nearly all, metrics from a Prometheus server into Splunk
 - HA and scaling of Splunk achievable with HTTP load balancers
Cons:
 - Must send to Splunk with same frequency as metrics are gathered into Prometheus

![](https://raw.githubusercontent.com/ltmon/splunk_modinput_prometheus/master/overview.png)

### Hybrid
All metrics gathered by the above methods are in a consistent format in Splunk, and reporting over them will be no different. Because of this, different ways of delivering metrics for different use cases could be implemented.

<png>

## Download

This add-on will be hosted at apps.splunk.com in the near future. It will be uploaded there when some further testing has been completed.

In the meantime, the latest build is available in the Github releases tab.

## Build

This assumes you have a relatively up-to-date Go build environment set up.

You will need some dependencies installed:

```
$ go get github.com/gogo/protobuf/proto
$ go get github.com/golang/snappy
$ go get github.com/prometheus/common/model
$ go get github.com/prometheus/prometheus/prompb
$ go get github.com/gobwas/glob
$ go get github.com/prometheus/prometheus/pkg/textparse
```

The "build" make target will build the modular input binaries, and copy tem into the correct place in `modinput_prometheus`, which forms the root of the Splunk app.

```
$ make build
```

## Install and configure

This add-on is installed just like any Splunk app: either through the web UI, deployment server or copying directly to $SPLUNK_HOME/etc/apps.

We recommend installing on a heavy forwarder, so the processing of events into metrics occurs at the collection point and not on indexers. The app is only tested on a heavy instance so far, but if you use a Universal Forwarder be sure to also install on your HFs/Indexers as there are index-time transforms to process the received metrics.

All available parameters are described in inputs.conf.spec.

### Static exporter

The most basic configuration will be to poll a Prometheus exporter.

e.g.

```
[prometheus://java-client-1]
URI = http://myhost:1234/metrics
index = prometheus
sourcetype = prometheus:metric
host = myhost
interval = 60
disabled = 0
```

The index should be a "metrics" type index. The sourcetype should be prometheus:metric, which is configured in the app to recognize the data format and convert it to Splunk metrics.

### Federate server

This configuration is to gather all metrics from a Prometheus server. The "match" string here matches all metrics, but you can learn more about how to configure metrics matching at: https://prometheus.io/docs/prometheus/latest/querying/basics/#instant-vector-selectors

```
[prometheus://prom-server-1]
URI = http://myhost:9090/federate
match = {__name__=~"..*"}
index = prometheus
sourcetype = prometheus:metric
host = myhost
interval = 60
disabled = 0
```

### Promtheus remote-write

Multiple input stanzas are required, but only one HTTP server is ever run. The individual inputs are distinguished by bearer tokens. A special `[prometheus]` sets up the HTTP server, and any other named input configures the specifics for that input itself.

e.g.

```
[prometheusrw]
port = 8098
maxClients = 10
disabled = 0

[prometheusrw://testing]
bearerToken = ABC123
index = prometheus
whitelist = *
sourcetype = prometheus:metric
disabled = 0

[prometheus://another]
bearerToken = DEF456
index = another_metrics_index
whitelist = net*
sourcetype = prometheus:metric
disabled = 0
```

This starts the HTTP listener on port 8098, and any metrics coming in with a bearer token of "ABC123" will be directed to the "testing" input. Not including a bearer token will result in a HTTP 401 (Unauthorized).

Although this configuration does allow some basic whitelist and blacklist behaviour within Splunk, it will be more efficient to do this on the Prometheus server using write_relabel_configs. An example is shown below.

In your Prometheus runtime YML file, ensure the following is set:

```yaml
  remote_write:
    - url: "http://<hostname>:8098"
      bearer_token: "ABC123"
      write_relabel_configs:
        - source_labels: [__name__]
          regex:         expensive.*
          action:        drop
```

Full details of available options are at: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cremote_write%3E

## Known Limitations

 - Only Linux on x86_64 is tested for now
 - Validation of configuration is non-existent -- incorrect config will not work with little indication as to why
 - Proper logging of the input execution is not yet implemented. You may or may get a log entry of any issues currently.
