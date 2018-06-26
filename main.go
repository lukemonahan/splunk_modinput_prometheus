package main

import (
        "fmt"
        "math"
        "strings"
        "io/ioutil"
        "net/http"
        "os"
        "log"
        "bytes"
        "encoding/xml"
        "strconv"

        "github.com/gogo/protobuf/proto"
        "github.com/golang/snappy"
        "github.com/prometheus/common/model"
        "github.com/prometheus/prometheus/prompb"
        "github.com/gobwas/glob"
)

// Structs to hold XML parsing of input from Splunk
type Input struct {
  XMLName xml.Name `xml:"input"`
  ServerHost string `xml:"server_host"`
  ServerURI string `xml:"server_uri"`
  SessionKey string `xml:"session_key"`
  CheckpointDir string `xml:"checkpoint_dir"`
  Configuration Configuration `xml:"configuration"`
}

type Configuration struct {
  XMLName xml.Name `xml:"configuration"`
  Stanzas []Stanza `xml:"stanza"`
}

type Stanza struct {
  XMLName xml.Name `xml:"stanza"`
  Params []Param `xml:"param"`
  Name string `xml:"name,attr"`
}

type Param struct {
  XMLName xml.Name `xml:"param"`
  Name string `xml:"name,attr"`
  Value string `xml:",chardata"`
}
// End XML structs

type InputConfig struct {
  ListenAddr string
  BearerToken string
  Whitelist []glob.Glob
  Blacklist []glob.Glob
  Index string
  Sourcetype string
  Host string
  MaxClients int
}

func main() {

        if len(os.Args) > 1 {
          if os.Args[1] == "--scheme" {
            fmt.Println(do_scheme())
          } else if os.Args[1] == "--validate-arguments" {
            validate_arguments()
          }
        } else {
          run()
        }

        return
}

func do_scheme() string {

  scheme := `<scheme>
      <title>Prometheus Remote Write</title>
      <description>Listen on a TCP port as a remote write endpoint for the Prometheus metrics server</description>
      <use_external_validation>false</use_external_validation>
      <streaming_mode>simple</streaming_mode>
      <use_single_instance>true</use_single_instance>
      <endpoint>
        <args>
          <arg name="listen_port">
            <title>HTTP port</title>
            <description>The port to run our HTTP server, which will be the remote write endpoint for Prometheus (default 8098)</description>
            <required_on_edit>false</required_on_edit>
            <required_on_create>false</required_on_create>
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
          <arg name="max_clients">
            <title>Concurrent clients</title>
            <description>The number of HTTP client connections to process at one time (default 10). Connections beyond this will be queued.</description>
            <required_on_edit>false</required_on_edit>
            <required_on_create>false</required_on_create>
          </arg>
      </endpoint>
    </scheme>`

  return scheme
}

func validate_arguments() {
  return
}

func run() error {

        // Parse arguments
        data, _ := ioutil.ReadAll(os.Stdin)
        var input Input
        xml.Unmarshal(data, &input)

        var globalConfig InputConfig
        configMap := make(map[string]InputConfig)

        for _, s := range input.Configuration.Stanzas {
          var inputConfig InputConfig
          for _, p := range s.Params {
            if p.Name == "listen_port" { inputConfig.ListenAddr = ":" + p.Value }
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
            if p.Name == "max_clients" {
              maxClients, error := strconv.Atoi(p.Value)
              if error != nil || maxClients <= 0 {
                inputConfig.MaxClients = 10
              } else {
                inputConfig.MaxClients = maxClients
              }
            }
            if p.Name == "bearer_token" { inputConfig.BearerToken = p.Value }
            if p.Name == "index" { inputConfig.Index = p.Value }
            if p.Name == "sourcetype" { inputConfig.Sourcetype = p.Value }
            if p.Name == "host" { inputConfig.Host = p.Value }

          }

          if s.Name == "prometheus://default" {
            globalConfig = inputConfig
          } else {
            configMap[inputConfig.BearerToken] = inputConfig
          }
        }

        // Semaphore to limit to maxClients concurrency
        sema := make(chan struct{}, globalConfig.MaxClients)

        // Output of metrics are sent to Splunk via log interface
        // This ensures parallel requests don't interleave, which can happen using stdout directly
        output := log.New(os.Stdout, "", 0)

        // Actual logging (goes to splunkd.log)
        //errLog := log.New(os.Stderr, "ERROR ", 0)
        //infoLog := log.New(os.Stderr, "INFO ", 0)
        //debugLog := log.New(os.Stderr, "DEBUG ", 0)


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
                defer func() { <-sema } ()

                // A buffer to build out metrics in for this request
                // Asynchronously dump to stdout (via logger) at end of request
                // We dump it all at once, as we may have index/sourcetype etc. directives
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
                          if (w.Match(string(m["__name__"]))) { whitelisted = true }
                        }

                        if !whitelisted { continue }

                        blacklisted := false
                        for _, b := range inputConfig.Blacklist {
                          if (b.Match(string(m["__name__"]))) { blacklisted = true }
                        }

                        if blacklisted { continue }

                        for _, s := range ts.Samples {
                                if math.IsNaN(s.Value) { continue } // Splunk won't accept NaN metrics
                                buffer.WriteString(fmt.Sprintf("%d %f %s\n", s.Timestamp, s.Value, m))
                        }
                }

                output.Print(buffer.String())
        })

        return http.ListenAndServe(globalConfig.ListenAddr, nil)
}
