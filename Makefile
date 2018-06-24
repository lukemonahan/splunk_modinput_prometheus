build: main.go
	go build -ldflags "-linkmode external -extldflags -static" main.go
	mv main modinput_prometheus/linux_x86_64/bin/prometheus
