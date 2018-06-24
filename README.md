# Prometheus Remote Write for Splunk

**WARNING**: This is a very early release and has undergone only limited testing. The version will be 1.0 when considered stable and complete.

Prometheus [prometheus.io](https://prometheus.io), a [Cloud Native Computing Foundation](https://cncf.io/) project, is a systems and service monitoring system. It collects metrics from configured targets at given intervals, evaluates rule expressions, displays the results, and can trigger alerts if some condition is observed to be true.

Splunk [splunk.com](https://www.splunk.com) is a platform for machine data analysis, providing real-time visibility, actionable insights and intelligence across all forms of machine data. Splunk Enterprise since version 7.0 includes the Metrics store for large-scale capture and analysis of time-series metrics alongside log data.

This Splunk add-on provides a bridge so that the Prometheus remote-write feature can continuously deliver metrics to a Splunk Enterprise system for long-term storage, analysis and integration with other data sources in Splunk. It is structured as a Splunk app that provides a modular input implementing the remote-write bridge. When installed and enabled, this add-on will add a new listening port to your Splunk server which can be the target for multiple Prometheus servers remote write.

## Architecture overview

![](https://raw.githubusercontent.com/ltmon/splunk_modinput_prometheus/master/overview.png)

## Download

This add-on will be hosted at apps.splunk.com in the near future. It will be uploaded there when some further testing has been completed.

In the meantime, the latest build is available in the Github releases tab.

## Build

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

You may get a warning about getaddrinfo, as libnss cannot be included in a static binary. This is fine to ignore.

## Install and configure

This add-on is installed just like any Splunk app: either through the web UI, deployment server or copying directly to $SPLUNK_HOME/etc/apps.

We recommend installing on a heavy forwarder, so the processing of events into metrics occurs at the collection point and not on indexers. The app is only tested on a heavy instance so far, but if you use a Universal Forwarder be sure to also install on your HFs/Indexers as there are index-time transforms to process the received metrics.

The input can be configured in Splunk web in the usual place, or in inputs.conf directly. A default input is configured upon install, but not enabled. The following configuration keys are available:

**listen_port**
The TCP port to listen on. Default 8098.

**whitelist**
A comma-separated list of glob patterns. Only metrics matching the patterns here will be forwarded on to Splunk. Default `*`.

**blacklist**
A comma-separated list of glob patterns. Metrics matching these patterns will not be forwarded to Splunk. These patterns are applied after the whitelist patterns and override them. Default empty.

**max_clients**
The maximum number of simultaneous HTTP requests the listener will process. More requests than this will be queued (the queue in unbounded). Default `10`.

A few notes on other defaults:

**index**
Defaults to an index called `prometheus`. Ensure whichever index you choose exists, and is a "metrics" type index.

**sourcetype**
Defaults to `prometheus:metrics`. This sourcetype is configured in props.conf and transforms.conf to convert correctly into a Splunk metric. Changing from this sourcetype will stop metrics conversion occurring unless you also make the required `props.conf` additions.

## Configure Prometheus

In your Prometheus runtime YML file, ensure the following is set:

```yaml
  remote_write:
    - url: "http://<hostname>:8098"
```

## Limitations

 - Only Linux on x86_64 is tested for now
 - TLS is not yet supported, and is targeted for future enhancement
 - No authentication or authorization to send data to the endpoint is available, but is also targeted for a future release
 - Splunk `host` field is static. Future releases will set this to the hostname/address of the sending Prometheus server.
