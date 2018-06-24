package main

import (
        "fmt"
        "math"
        "strings"
        "io/ioutil"
        "bufio"
        "net/http"
        "os"
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
  Stanza Stanza `xml:"stanza"`
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
      <endpoint>
        <args>
          <arg name="listen_port">
            <title>HTTP port</title>
            <description>The port to run our HTTP server, which will be the remote write endpoint for Prometheus (default 8098)</description>
            <validation>is_avail_tcp_port('listen_port')</validation>
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

        var (
            listenAddr            = ":8098"
            whitelistStr          = "*"
            blacklistStr          = ""
            maxClients            = 10
        )

        // Parse arguments
        data, _ := ioutil.ReadAll(os.Stdin)
        var input Input
        xml.Unmarshal(data, &input)

        for _, p := range input.Configuration.Stanza.Params {
          if p.Name == "listen_port" { listenAddr = ":" + p.Value }
          if p.Name == "whitelist" { whitelistStr = p.Value }
          if p.Name == "blacklist" { blacklistStr = p.Value }
          if p.Name == "max_clients" {
            maxClients, error := strconv.Atoi(p.Value)
            if error != nil || maxClients <= 0 { maxClients = 10 }
          }
        }

        var whitelist []glob.Glob
        for _, w := range strings.Split(whitelistStr, ",") {
          whitelist = append(whitelist, glob.MustCompile(w))
        }

        var blacklist []glob.Glob
        for _, b := range strings.Split(blacklistStr, ",") {
          blacklist = append(blacklist, glob.MustCompile(b))
        }

        // Semaphore to limit to maxClients concurrency
        sema := make(chan struct{}, maxClients)

        // Buffer stdout
        f := bufio.NewWriter(os.Stdout)

        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

                // This will queue a client if > maxClients are processing
                sema <- struct{}{}
                defer func() { <-sema } ()

                // Flush metrics back to Splunk at end of func
                defer f.Flush()

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
                        for _, w := range whitelist {
                          if (w.Match(string(m["__name__"]))) { whitelisted = true }
                        }

                        if !whitelisted { continue }

                        blacklisted := false
                        for _, b := range blacklist {
                          if (b.Match(string(m["__name__"]))) { blacklisted = true }
                        }

                        if blacklisted { continue }

                        for _, s := range ts.Samples {
                                if math.IsNaN(s.Value) { continue } // Splunk won't accept NaN metrics
                                fmt.Fprintln(f, s.Timestamp, s.Value, m)
                        }
                }
        })

        return http.ListenAndServe(listenAddr, nil)
}
