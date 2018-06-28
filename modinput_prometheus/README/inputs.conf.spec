[prometheus]
port = <number>
maxClients = <number>
enableTLS = <0|1>
certFile = <path>
keyFile = <path>

[prometheus://<name>]
bearerToken = <string>
whitelist = <glob pattern>,<glob pattern>,...
blacklist = <glob pattern>,<glob pattern>,...
