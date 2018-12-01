FROM golang:1.11 as builder

RUN apt-get update; DEBIAN_FRONTEND=noninteractive apt-get install -y git
WORKDIR /go/src/splunk_modinput_prometheus
COPY . /go/src/splunk_modinput_prometheus

RUN go get github.com/gogo/protobuf/proto
RUN go get github.com/golang/snappy
RUN go get github.com/prometheus/common/model
RUN go get github.com/prometheus/prometheus/prompb
RUN go get github.com/gobwas/glob
RUN go get github.com/prometheus/prometheus/pkg/textparse

FROM debian:stretch-slim
COPY --from=builder /go/src/splunk_modinput_prometheus/modinput_prometheus/ /opt/modinput_prometheus

