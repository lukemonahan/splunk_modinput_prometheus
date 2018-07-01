[prometheusrw]
port = <number>
maxClients = <number>
enableTLS = <0|1>
certFile = <path>
keyFile = <path>

[prometheusrw://<name>]
bearerToken = <string>
whitelist = <glob pattern>,<glob pattern>,...
blacklist = <glob pattern>,<glob pattern>,...

[prometheus://<name>]
URI = <URI>
match = <match expression>
