# Prometheus Remote Write for Splunk

**WARNING**: This is a very early release and has undergone only limited testing. The version will be 1.0 when considered stable and complete.

**DOUBLE WARNING**: Building from Git will currently not match these documents. Latest release (currently 0.4) will match this document.

Prometheus [prometheus.io](https://prometheus.io), a [Cloud Native Computing Foundation](https://cncf.io/) project, is a systems and service monitoring system. It collects metrics from configured targets at given intervals, evaluates rule expressions, displays the results, and can trigger alerts if some condition is observed to be true.

Splunk [splunk.com](https://www.splunk.com) is a platform for machine data analysis, providing real-time visibility, actionable insights and intelligence across all forms of machine data. Splunk Enterprise since version 7.0 includes the Metrics store for large-scale capture and analysis of time-series metrics alongside log data.

This Splunk add-on provides a bridge so that the Prometheus remote-write feature can continuously deliver metrics to a Splunk Enterprise system for long-term storage, analysis and integration with other data sources in Splunk. It is structured as a Splunk app that provides a modular input implementing the remote-write bridge. When installed and enabled, this add-on will add a new listening port to your Splunk server which can be the target for multiple Prometheus servers remote write.

It has been designed to mimic the Splunk HTTP Event Collector from a configuration standpoint, however the endpoint is much simpler as it only support Prometheus remote-write. The HEC is not used as Prometheus remote-write requires Speedy compression and Protocol Buffer encoding, both of which do not work with the HEC.

## Architecture overview

![](https://raw.githubusercontent.com/ltmon/splunk_modinput_prometheus/master/overview.png)

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
```

The "build" make target will build the modular input binary statically, and copy it into the correct place in `modinput_prometheus`, which forms the root of the Splunk app.

```
$ make build
```

You may get a warning about getaddrinfo, as libnss cannot be included in a static binary. This warning is fine to ignore.

## Install and configure

This add-on is installed just like any Splunk app: either through the web UI, deployment server or copying directly to $SPLUNK_HOME/etc/apps.

We recommend installing on a heavy forwarder, so the processing of events into metrics occurs at the collection point and not on indexers. The app is only tested on a heavy instance so far, but if you use a Universal Forwarder be sure to also install on your HFs/Indexers as there are index-time transforms to process the received metrics.

Multiple input stanzas are required, but only one HTTP server is ever run. The individual inputs are distinguished by bearer tokens. A special `[prometheus]` sets up the HTTP server, and any other named input configures the specifics for that input itself.

e.g.

```
[prometheus]
port = 8098
maxClients = 10
disabled = 0

[prometheus://testing]
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

The input can be configured in Splunk web in the usual place, or in inputs.conf directly.

### Global input parameters

These parameters should be set under the [prometheus] stanza.

**port**
The TCP port to listen on. Default 8098.

**maxClients**
The maximum number of simultaneous HTTP requests the listener will process. More requests than this will be queued (the queue in unbounded). Default `10`.

**enableTLS**
Listen on TLS (i.e. https://<server>:<port>)

**certFile**
Full path to certFile. $SPLUNK_HOME is respected. Any CA certificate should be contatenated.

**keyFile**
Full path to keyFile. Passwordless only supported for now. $SPLUNK_HOME is respected.

**disabled**
If set to true (or 1 etc.) no prometheus inputs will function

### Specific input parameters

**bearerToken**
The token that will identify incoming requests to this input

**whitelist**
A comma-separated list of glob patterns. Only metrics matching the patterns here will be forwarded on to Splunk. Default `*`.

**blacklist**
A comma-separated list of glob patterns. Metrics matching these patterns will not be forwarded to Splunk. These patterns are applied after the whitelist patterns and override them. Default empty.

**sourcetype**
Should usually be `prometheus:metric`. If you wish to use a different sourcetype, then you will need to set up metric parsing in props.conf for your new sourcetype.

**index**
Needs to be a "metrics" index

**host**
Can be set statically for each input

## Configure Prometheus

In your Prometheus runtime YML file, ensure the following is set:

```yaml
  remote_write:
    - url: "http://<hostname>:8098"
      bearer_token: "ABC123"
```

Full details of available options are at: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cremote_write%3E

## Known Limitations

 - Only Linux on x86_64 is tested for now
 - Validation of configuration is non-existent -- incorrect config will not work with little indication as to why
 - Proper logging of the input execution is not yet implemented. You may or may get a log entry of any issues.
