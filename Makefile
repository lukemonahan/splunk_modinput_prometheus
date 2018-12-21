build:
	make -B prometheusrw
	make -B prometheus

prometheusrw: prometheusrw/prometheusrw.go
	go build -o ./out/prometheusrw ./prometheusrw/prometheusrw.go

	mv ./out/prometheusrw modinput_prometheus/linux_x86_64/bin/

prometheus: prometheus/prometheus.go
	go build  -o ./out/prometheus ./prometheus/prometheus.go
	mv ./out/prometheus modinput_prometheus/linux_x86_64/bin/

package:
	make build
	tar cvfz modinput_prometheus.tar.gz modinput_prometheus

# To use the validate target, install a Python venv in this directory, and install splunk-appinspect within it
# http://dev.splunk.com/view/appinspect/SP-CAAAFAW#installinvirtualenv
validate:
	make package
	bash -c 'source venv/bin/activate && splunk-appinspect inspect ./modinput_prometheus.tar.gz'

