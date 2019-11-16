package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gobwas/glob"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
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

type feed struct {
	XMLName xml.Name `xml:"feed"`
	Keys    []key    `xml:"entry>content>dict>key"`
}
type key struct {
	XMLName xml.Name `xml:"key"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

// End XML structs

// Structs store final config
type inputConfig struct {
	BearerToken      string
	Whitelist        []glob.Glob
	Blacklist        []glob.Glob
	Index            string
	Sourcetype       string
	Host             string
	MetricNamePrefix string // Add a custom prefix to metric name
	MetricNameParse  bool   // Parse metric according to splunk prefix
}

type globalConfig struct {
	ListenAddr string
	MaxClients int
	Disabled   bool
	EnableTLS  bool
	CertFile   string
	KeyFile    string
}

// End config structs

func main() {

	if len(os.Args) > 1 {
		if os.Args[1] == "--scheme" {
			fmt.Println(doScheme())
		} else if os.Args[1] == "--validate-arguments" {
			validateArguments()
		}
	} else {
		log.Fatal(run())
	}

	return
}

func doScheme() string {

	scheme := `<scheme>
      <title>Prometheus Remote Write</title>
      <description>Listen on a TCP port as a remote write endpoint for the Prometheus metrics server</description>
      <use_external_validation>false</use_external_validation>
      <streaming_mode>simple</streaming_mode>
      <use_single_instance>true</use_single_instance>
      <endpoint>
          <arg name="bearerToken">
            <title>Bearer token</title>
            <description>A token configured in Prometheus to send via the Authorization header</description>
            <required_on_edit>true</required_on_edit>
            <required_on_create>true</required_on_create>
          </arg>
          <arg name="whitelist">
            <title>Whitelist</title>
            <description>A comma-separated list of glob patterns to match metric names and index (default *)</description>
            <required_on_edit>false</required_on_edit>
            <required_on_create>false</required_on_create>
          </arg>
          <arg name="blacklist">
            <title>Blacklist</title>
            <description>A comma-separated list of glob patterns to match metric names and prevent indexing (default empty). Applied after whitelisting.</description>
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

func config() (globalConfig, map[string]inputConfig) {

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var input input
	err = xml.Unmarshal(data, &input)

	if err != nil {
		log.Fatal(err)
	}

	configMap := make(map[string]inputConfig)

	for _, s := range input.Configuration.Stanzas {

		var inputConfig inputConfig

		inputConfig.MetricNameParse = true
		inputConfig.MetricNamePrefix = "prometheus."

		for _, p := range s.Params {
			if p.Name == "whitelist" {
				for _, w := range strings.Split(p.Value, ",") {
					inputConfig.Whitelist = append(inputConfig.Whitelist, glob.MustCompile(w))
				}
			}
			if p.Name == "blacklist" {
				for _, b := range strings.Split(p.Value, ",") {
					inputConfig.Blacklist = append(inputConfig.Blacklist, glob.MustCompile(b))
				}
			}
			if p.Name == "bearerToken" {
				inputConfig.BearerToken = p.Value
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
			if p.Name == "metricNamePrefix" {
				inputConfig.MetricNamePrefix = p.Value
			}
			if p.Name == "metricNameParse" {
				inputConfig.MetricNameParse, _ = strconv.ParseBool(p.Value)
			}
		}


		configMap[inputConfig.BearerToken] = inputConfig
	}
	// Default global config
	var globalConfig globalConfig
	globalConfig.ListenAddr = ":8098"
	globalConfig.MaxClients = 10
	globalConfig.Disabled = true
	globalConfig.EnableTLS = false

	// Get the global configuration
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", input.ServerURI+"/services/configs/inputs/prometheusrw", nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Authorization", "Splunk "+input.SessionKey)
	response, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Parse the global configuration
	var feed feed
	xml.Unmarshal(body, &feed)
	for _, k := range feed.Keys {
		if k.Name == "disabled" {
			globalConfig.Disabled, _ = strconv.ParseBool(k.Value)
		}
		if k.Name == "port" {
			port, _ := strconv.Atoi(k.Value)
			globalConfig.ListenAddr = fmt.Sprintf(":%d", port)
		}
		if k.Name == "maxClients" {
			maxClients, error := strconv.Atoi(k.Value)
			if error != nil || maxClients <= 0 {
				globalConfig.MaxClients = 10
			} else {
				globalConfig.MaxClients = maxClients
			}
		}
		if k.Name == "enableTLS" {
			globalConfig.EnableTLS, _ = strconv.ParseBool(k.Value)
		}
		if k.Name == "certFile" {
			globalConfig.CertFile = strings.Replace(k.Value, "$SPLUNK_HOME", os.Getenv("SPLUNK_HOME"), -1)
		}
		if k.Name == "keyFile" {
			globalConfig.KeyFile = strings.Replace(k.Value, "$SPLUNK_HOME", os.Getenv("SPLUNK_HOME"), -1)
		}
	}
	response.Body.Close()

	return globalConfig, configMap
}

func run() error {

	// Output of metrics are sent to Splunk via log interface
	// This ensures parallel requests don't interleave, which can happen using stdout directly
	output := log.New(os.Stdout, "", 0)

	// Actual logging (goes to splunkd.log)
	//infoLog := log.New(os.Stderr, "INFO ", 0)
	//debugLog := log.New(os.Stderr, "DEBUG ", 0)
	//errLog := log.New(os.Stderr, "ERROR ", 0)

	globalConfig, configMap := config()

	if globalConfig.Disabled == true {
		log.Fatal("Prometheus input globally disabled")
	}

	// Semaphore to limit to maxClients concurrency
	sema := make(chan struct{}, globalConfig.MaxClients)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Get the bearer token and corresponding config
		bearerToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		if _, ok := configMap[bearerToken]; !ok {
			http.Error(w, "Bearer token not recognized. Please contact your Splunk admin.", http.StatusUnauthorized)
			return
		}

		inputConfig := configMap[bearerToken]

		// This will queue a client if > maxClients are processing
		sema <- struct{}{}
		defer func() { <-sema }()

		// A buffer to build out metrics in for this request
		// We dump it all at once, as we may have index/sourcetype etc. directives and we can't have them separated from the metrics they effect by another request
		var buffer bytes.Buffer

		buffer.WriteString(fmt.Sprintf("***SPLUNK*** index=%s sourcetype=%s host=%s\n", inputConfig.Index, inputConfig.Sourcetype, inputConfig.Host))

		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, ts := range req.Timeseries {

			m := make(model.Metric, len(ts.Labels))

			for _, l := range ts.Labels {
				m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
			}

			whitelisted := false
			for _, w := range inputConfig.Whitelist {
				if w.Match(string(m["__name__"])) {
					whitelisted = true
				}
			}

			if !whitelisted {
				continue
			}

			blacklisted := false
			for _, b := range inputConfig.Blacklist {
				if b.Match(string(m["__name__"])) {
					blacklisted = true
				}
			}

			if blacklisted {
				continue
			}

			if inputConfig.MetricNameParse {
				m["__name__"] = formatMetricLabelValue(string(m["__name__"]), inputConfig.MetricNamePrefix)
			}

			for _, s := range ts.Samples {
				if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) {
					continue
				} // Splunk won't accept NaN metrics etc.
				buffer.WriteString(fmt.Sprintf("%s %f %d\n", m, s.Value, s.Timestamp))
			}
		}

		output.Print(buffer.String())
		buffer.Truncate(0)
	})

	if globalConfig.EnableTLS == true {
		return http.ListenAndServeTLS(globalConfig.ListenAddr, globalConfig.CertFile, globalConfig.KeyFile, nil)
	} else {
		return http.ListenAndServe(globalConfig.ListenAddr, nil)
	}
}

func formatMetricLabelValue(value string, prefix string) model.LabelValue {
	s := []string{}
	s = append(s, prefix)
	s = append(s, regexp.MustCompile("_").ReplaceAllString(value, "."))
	return model.LabelValue(strings.Join(s, ""))
}
