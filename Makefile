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

# To use the validate taget, install a Python venv in this directory, and install splunk-appinspect within it
# http://dev.splunk.com/view/appinspect/SP-CAAAFAW#installinvirtualenv
validate:
	make package
	bash -c 'source venv/bin/activate && splunk-appinspect inspect ./modinput_prometheus.tar.gz'
