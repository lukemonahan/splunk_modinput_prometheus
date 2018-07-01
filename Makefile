build:
	make prometheusrw
	make prometheus

prometheusrw: prometheusrw.go
	go build -ldflags "-linkmode external -extldflags -static" prometheusrw.go
	mv prometheusrw modinput_prometheus/linux_x86_64/bin/

prometheus: prometheus.go
	go build -ldflags "-linkmode external -extldflags -static" prometheus.go
	mv prometheus modinput_prometheus/linux_x86_64/bin/
