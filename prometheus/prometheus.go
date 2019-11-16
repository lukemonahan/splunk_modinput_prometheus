package main

import (
	"bufio"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"regexp"

	"github.com/prometheus/prometheus/pkg/textparse"
)

// Structs to hold XML parsing of input from Splunk
type input struct {
	XMLName       xml.Name      `xml:"input"`
	ServerHost    string        `xml:"server_host"`
	ServerURI     string        `xml:"server_uri"`
	SessionKey    string        `xml:"session_key"`
	CheckpointDir string        `xml:"checkpoint_dir"`
	Configuration configuration `xml:"configuration"`
}

type configuration struct {
	XMLName xml.Name `xml:"configuration"`
	Stanzas []stanza `xml:"stanza"`
}

type stanza struct {
	XMLName xml.Name `xml:"stanza"`
	Params  []param  `xml:"param"`
	Name    string   `xml:"name,attr"`
}

type param struct {
	XMLName xml.Name `xml:"param"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

// End XML structs

// Struct to store final config
type inputConfig struct {
	URI                string
	Match              []string
	InsecureSkipVerify bool
	Index              string
	Sourcetype         string
	Host               string
	MetricNamePrefix string // Add a custom prefix to metric name
	MetricNameParse  bool   // Parse metric according to splunk prefix
}

var (
	// http://docs.splunk.com/Documentation/Splunk/7.2.1/AdvancedDev/ModInputsLog
	logInfo  = log.New(os.Stderr, "INFO  (prometheus) ", 0)
	logDebug = log.New(os.Stderr, "DEBUG (prometheus) ", 0)
	logError = log.New(os.Stderr, "ERROR (prometheus) ", 0)

	/*
		    We use match separator ";" for matchSeparator as "," (coma) is reserved to match a label value,
			or to match label values against regular expressions.

			The following label matching operators exist:

		    =: Select labels that are exactly equal to the provided string.
		    !=: Select labels that are not equal to the provided string.
		    =~: Select labels that regex-match the provided string (or substring).
		    !~: Select labels that do not regex-match the provided string (or substring).

			Please visit:
			https://prometheus.io/docs/prometheus/latest/querying/basics/#operators
			for more information on label matching.
	*/

	matchSeparator = ";"
)

func main() {

	/*

		TESTING MODULAR INPUTS IN DEV ENVIRONMENT

		In order to local test modular inputs please do the following:
		http://docs.splunk.com/Documentation/Splunk/7.2.1/AdvancedDev/ModInputsDevTools

		1. Grab the output from you local stanza

		Example for local inputs.conf:

		[prometheus://example-federate]
		URI = http://localhost:9090/federate
		match = {__name__=~"..*"}
		index = prometheus
		sourcetype = prometheus:metric
		interval = 60
		disabled = 1

		# Extract stdin for local testing:
		$./bin/splunk cmd splunkd print-modinput-config prometheus prometheus://example-federate

		<?xml version="1.0" encoding="UTF-8"?>
		<input>
		<server_host>localhost</server_host>
		<server_uri>https://127.0.0.1:8089</server_uri>
		<session_key>^gbQv^_I4sBVjoMX6Lo7K6sxK0rpKsdMIZmTecbHdm2L0tKJ1gwl8ctZxI5V^ZX6qTUyAyFEg3FKf3iuZjZoy9KGQ7nmS5WDemZyo_VHVKA7q9q9ecHMrXr</session_key>
		<checkpoint_dir>/opt/splunk/var/lib/splunk/modinputs/prometheus</checkpoint_dir>
		<configuration>
			<stanza name="prometheus://example-federate">
			<param name="URI">http://localhost:9090/federate</param>
			<param name="disabled">0</param>
			<param name="host">localhost</param>
			<param name="index">prometheus</param>
			<param name="interval">60</param>
			<param name="match">{__name__=~"..*"}</param>
			<param name="sourcetype">prometheus:metric</param>
			</stanza>
		</configuration>
		</input>

		2. Write your stanza into a xml file eg: tester.xml

		3. Use your stanza tester.xml file as stdin for you modular input:

		4. cat tester.xml | go run prometheus.go

	*/

	if len(os.Args) > 1 {
		if os.Args[1] == "--scheme" {
			fmt.Println(doScheme())
		} else if os.Args[1] == "--validate-arguments" {
			validateArguments()
		}
	} else {
		run()
	}

	return
}

func doScheme() string {

	scheme := `<scheme>
      <title>Prometheus</title>
      <description>Scrapes a Prometheus endpoint, either directly or via Prometheus federation</description>
      <use_external_validation>false</use_external_validation>
      <streaming_mode>simple</streaming_mode>
      <use_single_instance>false</use_single_instance>
      <endpoint>
          <arg name="URI">
            <title>Metrics URI</title>
            <description>A Prometheus exporter endpoint</description>
            <required_on_edit>true</required_on_edit>
            <required_on_create>true</required_on_create>
          </arg>
					<arg name="match">
						<title>Match filter</title>
						<description>A comma-delimited list of Prometheus "match" expressions: only functional and required for /federate endpoints</description>
						<required_on_edit>false</required_on_edit>
						<required_on_create>false</required_on_create>
					</arg>
					<arg name="insecureSkipVerify">
						<title>Skip certificate verification</title>
						<description>If the endpoint is HTTPS, this setting controls whether to skip verification of the server certificate or not</description>
						<required_on_edit>false</required_on_edit>
						<required_on_create>false</required_on_create>
					</arg>
					<arg name="metricNameParse">
						<title>Parse metric names</title>
						<description>Rewrite the name of the Prometheus metric into a more Splunk suitable format. Default true.</description>
						<required_on_edit>false</required_on_edit>
						<required_on_create>false</required_on_create>
					</arg>
					<arg name="metricNamePrefix">
						<title>Metric name prefix</title>
						<description>Prefix all metric names with this value. Default "prometheus.".</description>
						<required_on_edit>false</required_on_edit>
						<required_on_create>false</required_on_create>
					</arg>
      </endpoint>
    </scheme>`

	return scheme
}

func validateArguments() {
	// Currently unused
	// Will be used to properly validate in future
	return
}

func config() inputConfig {

	data, _ := ioutil.ReadAll(os.Stdin)
	var input input
	xml.Unmarshal(data, &input)

	var inputConfig inputConfig

	inputConfig.MetricNameParse = true
	inputConfig.MetricNamePrefix = "prometheus."

	for _, s := range input.Configuration.Stanzas {
		for _, p := range s.Params {
			if p.Name == "URI" {
				inputConfig.URI = p.Value
			}
			if p.Name == "insecureSkipVerify" {
				inputConfig.InsecureSkipVerify, _ = strconv.ParseBool(p.Value)
			}
			if p.Name == "index" {
				inputConfig.Index = p.Value
			}
			if p.Name == "sourcetype" {
				inputConfig.Sourcetype = p.Value
			}
			if p.Name == "host" {
				inputConfig.Host = p.Value
			}
			if p.Name == "match" {
				for _, m := range strings.Split(p.Value, matchSeparator) {
					inputConfig.Match = append(inputConfig.Match, m)
				}
			}
			if p.Name == "metricNamePrefix" {
				inputConfig.MetricNamePrefix = p.Value
			}
			if p.Name == "metricNameParse" {
				inputConfig.MetricNameParse, _ = strconv.ParseBool(p.Value)
			}
		}
	}

	return inputConfig
}

func run() {

	var inputConfig = config()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: inputConfig.InsecureSkipVerify},
	}

	client := &http.Client{Transport: tr, Timeout: time.Duration(500000 * time.Microsecond)}

	req, err := http.NewRequest("GET", inputConfig.URI, nil)

	if err != nil {
		log.Fatal("Request error", err)
	}

	q := req.URL.Query()
	for _, m := range inputConfig.Match {
		q.Add("match[]", m)
	}
	req.URL.RawQuery = q.Encode()

	// Debug request req.URL
	logDebug.Print(req.URL)

	// Current timestamp in millis, used if response has no timestamps
	now := time.Now().UnixNano() / int64(time.Millisecond)

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	// Output buffer
	output := bufio.NewWriter(os.Stdout)
	defer output.Flush()

	// Need to parse metrics out of body individually to convert from scientific to decimal etc. before handing to Splunk
	contentType := resp.Header.Get("Content-Type")
	p := textparse.New(body, contentType)

	for {
		et, err := p.Next()

		if err != nil {
			if err == io.EOF {
				break
			} else {
				continue
			}
		}

		// Only care about the actual metric series in Splunk for now
		if et == textparse.EntrySeries {
			b, ts, val := p.Series()

			if ts != nil {
				now = *ts
			}

			if math.IsNaN(val) || math.IsInf(val, 0) {
				continue
			} // Splunk won't accept NaN metrics etc.

			if inputConfig.MetricNameParse {
				b = []byte(formatMetricLabelValue(string(b), inputConfig.MetricNamePrefix))
			}

			output.WriteString(fmt.Sprintf("%s %f %d\n", b, val, now))
		}
	}

	return
}

func formatMetricLabelValue(value string, prefix string) string {
	s := []string{}
	s = append(s, prefix)
	s = append(s, regexp.MustCompile("_").ReplaceAllString(value, "."))
	return strings.Join(s, "")
}
