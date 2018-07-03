build:
	make prometheusrw
	make prometheus

prometheusrw: prometheusrw.go
	go build prometheusrw.go
	mv prometheusrw modinput_prometheus/linux_x86_64/bin/

prometheus: prometheus.go
	go build prometheus.go
	mv prometheus modinput_prometheus/linux_x86_64/bin/

package:
	make build
	tar cvfz modinput_prometheus.tar.gz modinput_prometheus
