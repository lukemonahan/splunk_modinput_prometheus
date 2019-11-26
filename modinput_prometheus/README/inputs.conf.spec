[prometheusrw]
* The global configuration of the Prometheus remote-write reciving port.
* Setting this input up is required, however you also need to set up a specific input for each bearer token that is sent from a Prometheus server.

port = <number>
* The TCP port we will listen to for connections from the Prometheus remote-write client. It will listen on all interfaces (":<port>"). The listener is on the root path (i.e. http://<host>:<port>/)

maxClients = <number>
* This dictates the number of requests that will be processed and converted to metrics in parallel. Any incoming remote write requests will be queued.

enableTLS = <0|1>
* Enable listening on HTTPS. If set, you need to set a certFile and keyFile.

certFile = <path>
* The server certificate. If a CA is required, it should be concatenated in this file. $SPLUNK_HOME environment substitution is supported.

keyFile = <path>
* The server certificate key. It must have no password. $SPLUNK_HOME environment substitution is supported.

[prometheusrw://<name>]
* A specific Promethes remote-write input. This requires the global [prometheusrw] input to also be enabled.

bearerToken = <string>
* Incoming Prometheus remote-write requests will be sent to this input if they have a matching bearer_token. Requests with no bearer token are treated as 401 Unauthorized.

whitelist = <glob pattern>,<glob pattern>,...
* A basic whitelist of metrics to ingest from the incoming stream.
* Comma-separated globs of matching metric names
* Must have at least one of "*" to match all metrics
* It is recommended to configure suppression of metrics in Prometheus itself using write_relabel_configs, however this configuration provides a way for the Splunk administrator to whitelist specific metrics also.

blacklist = <glob pattern>,<glob pattern>,...
* A basic whitelist of metrics to discared from the incoming stream.
* Comma-separated globs of matching metric names
* Applied after the whitelist
* It is recommended to configure suppression of metrics in Prometheus itself using write_relabel_configs, however this configuration provides a way for the Splunk administrator to blacklist specific metrics also.

metricNamePrefix = <string>
* A prefix for all metric names from the present stanza
* Use a ending "." in order to have an extra level on metric name tree display (eg: DEV.)

metricNameParse = <bool>
* A parser from prometheus default metric name separated by a "_" to a Splunk metric name separated by a "."
* After activation your prometheus metrics should display in a tree folded manner inside Splunk metric dashboard.

[prometheus://<name>]
* An outgoing connection to a Prometheus server (e.g. federated) or to a Prometheus exporter
* Any metrics series found at this endpoint will be converted to Splunk metrics and indexed

URI = <URI>
* The full URI of the exporter to connect to

match = <match expression>,<match expression>,...
* Match expressions, semicolon separated
* At least one valid match expression must be included if you are connecting to a Prometheus federate endpoint
* They are usually ignored for most exporters, however

insecureSkipVerify = <0|1>
* If the URI is HTTPS, this controls whether the server certificate must be verified in order to continue

username = <string>
* Provide an optional username to be used with the Prometheus federate endpoint

password = <string>
* Provide an optional password to be used with the Prometheus federate endpoint
