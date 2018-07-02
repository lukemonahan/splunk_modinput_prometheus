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

	"github.com/prometheus/prometheus/pkg/textparse"
)

// Structs to hold XML parsing of input from Splunk
type Input struct {
	XMLName       xml.Name      `xml:"input"`
	ServerHost    string        `xml:"server_host"`
	ServerURI     string        `xml:"server_uri"`
	SessionKey    string        `xml:"session_key"`
	CheckpointDir string        `xml:"checkpoint_dir"`
	Configuration Configuration `xml:"configuration"`
}

type Configuration struct {
	XMLName xml.Name `xml:"configuration"`
	Stanzas []Stanza `xml:"stanza"`
}

type Stanza struct {
	XMLName xml.Name `xml:"stanza"`
	Params  []Param  `xml:"param"`
	Name    string   `xml:"name,attr"`
}

type Param struct {
	XMLName xml.Name `xml:"param"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

// End XML structs

// Struct to store final config
type InputConfig struct {
	URI        string
	Match      []string
	InsecureSkipVerify bool
	Index      string
	Sourcetype string
	Host       string
}

func main() {

	if len(os.Args) > 1 {
		if os.Args[1] == "--scheme" {
			fmt.Println(DoScheme())
		} else if os.Args[1] == "--validate-arguments" {
			ValidateArguments()
		}
	} else {
		Run()
	}

	return
}

func DoScheme() string {

	scheme := `<scheme>
      <title>Prometheus</title>
      <description>Scrapes a Prometheus endpoint, either directly or via Prometheus federation</description>
      <use_external_validation>true</use_external_validation>
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
      </endpoint>
    </scheme>`

	return scheme
}

func ValidateArguments() {
	// Currently unused
	// Will be used to properly validate in future
	return
}

func Config() InputConfig {

	data, _ := ioutil.ReadAll(os.Stdin)
	var input Input
	xml.Unmarshal(data, &input)

	var inputConfig InputConfig

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
				for _, m := range strings.Split(p.Value, ",") {
					inputConfig.Match = append(inputConfig.Match, m)
				}
			}
		}
	}

	return inputConfig
}

func Run() {

	var inputConfig = Config()

	tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: inputConfig.InsecureSkipVerify},
  }

	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", inputConfig.URI, nil)

	if err != nil {
		log.Fatal(err)
	}

	q := req.URL.Query()
	for _, m := range inputConfig.Match {
		q.Add("match[]", m)
	}
	req.URL.RawQuery = q.Encode()

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
	p := textparse.New(body)

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

			if math.IsNaN(val) || math.IsInf(val, 0) {
				continue
			} // Splunk won't accept NaN metrics etc.
			output.WriteString(fmt.Sprintf("%s %f %d\n", b, val, *ts))
		}
	}

	return
}
