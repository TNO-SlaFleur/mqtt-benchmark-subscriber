package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/GaryBoone/GoStats/stats"
)

type Message struct {
    Payload Payload
    ReceivedAt int64
}

type Payload struct {
    GeneratedAt int64
    ClientId    int
    MessageId   int
}

// RunResults describes results of a single client / run
type RunResults struct {
	ID          int     `json:"id"`
	Successes   int64   `json:"successes"`
	RunTime     float64 `json:"run_time"`
	MsgTimeMin  float64 `json:"msg_time_min"`
	MsgTimeMax  float64 `json:"msg_time_max"`
	MsgTimeMean float64 `json:"msg_time_mean"`
	MsgTimeStd  float64 `json:"msg_time_std"`
	MsgsPerSec  float64 `json:"msgs_per_sec"`
	Duplicates  int64   `json:"duplicates"`
}

// TotalResults describes results of all clients / runs
type TotalResults struct {
	Ratio           float64 `json:"ratio"`
	Successes       int64   `json:"successes"`
	TotalRunTime    float64 `json:"total_run_time"`
	AvgRunTime      float64 `json:"avg_run_time"`
	MsgTimeMin      float64 `json:"msg_time_min"`
	MsgTimeMax      float64 `json:"msg_time_max"`
	MsgTimeMeanAvg  float64 `json:"msg_time_mean_avg"`
	MsgTimeMeanStd  float64 `json:"msg_time_mean_std"`
	TotalMsgsPerSec float64 `json:"total_msgs_per_sec"`
	AvgMsgsPerSec   float64 `json:"avg_msgs_per_sec"`
	Duplicates      int64   `json:"duplicates"`
}

// JSONResults are used to export results as a JSON document
type JSONResults struct {
	Runs   []*RunResults `json:"runs"`
	Totals *TotalResults `json:"totals"`
}

func main() {
	var (
		broker       = flag.String("broker", "tcp://localhost:1883", "MQTT broker endpoint as scheme://host:port")
		topic        = flag.String("topic", "/test", "MQTT topic for outgoing messages")
		username     = flag.String("username", "", "MQTT client username (empty if auth disabled)")
		password     = flag.String("password", "", "MQTT client password (empty if auth disabled)")
		qos          = flag.Int("qos", 1, "QoS for published messages")
		count        = flag.Int64("count", 100, "Number of messages to receive per client")
		clients      = flag.Int("clients", 10, "Number of clients to start")
		format       = flag.String("format", "text", "Output format: text|json")
		quiet        = flag.Bool("quiet", false, "Suppress logs while running")
		clientPrefix = flag.String("client-prefix", "mqtt-benchmark", "MQTT client id prefix (suffixed with '-<client-num>'")
		clientCert   = flag.String("client-cert", "", "Path to client certificate in PEM format")
		clientKey    = flag.String("client-key", "", "Path to private clientKey in PEM format")
	)

	flag.Parse()
    if *clients < 1 {
		log.Fatalf("Invalid arguments: number of clients should be > 1, given: %v", *clients)
	}

	if *count < 1 {
        log.Fatalf("Invalid arguments: messages count should be > 1, given: %v", *count)
    }

	if *clientCert != "" && *clientKey == "" {
		log.Fatal("Invalid arguments: private clientKey path missing")
	}

	if *clientCert == "" && *clientKey != "" {
		log.Fatalf("Invalid arguments: certificate path missing")
	}

	var tlsConfig *tls.Config
	if *clientCert != "" && *clientKey != "" {
		tlsConfig = generateTLSConfig(*clientCert, *clientKey)
	}

	resCh := make(chan *RunResults)
	start := time.Now()
	for i := 0; i < *clients; i++ {
		if !*quiet {
			log.Println("Starting client ", i)
		}
		c := &Client{
			ID:          i,
			ClientID:    *clientPrefix,
			BrokerURL:   *broker,
			BrokerUser:  *username,
			BrokerPass:  *password,
			MsgTopic:    *topic,
			ReceiveCount:    *count,
			MsgQoS:      byte(*qos),
			Quiet:       *quiet,
			TLSConfig:   tlsConfig,
		}
		go c.Run(resCh)
	}

	// collect the results
	results := make([]*RunResults, *clients)
	for i := 0; i < *clients; i++ {
		results[i] = <-resCh
	}
	totalTime := time.Since(start)
	totals := calculateTotalResults(results, totalTime, *clients)

	// print stats
	printResults(results, totals, *format)
}

func calculateTotalResults(results []*RunResults, totalTime time.Duration, sampleSize int) *TotalResults {
	totals := new(TotalResults)
	totals.TotalRunTime = totalTime.Seconds()

	msgTimeMeans := make([]float64, len(results))
	msgsPerSecs := make([]float64, len(results))
	runTimes := make([]float64, len(results))
	bws := make([]float64, len(results))

	totals.MsgTimeMin = results[0].MsgTimeMin
	for i, res := range results {
		totals.Successes += res.Successes
		totals.TotalMsgsPerSec += res.MsgsPerSec
		totals.Duplicates += res.Duplicates

		if res.MsgTimeMin < totals.MsgTimeMin {
			totals.MsgTimeMin = res.MsgTimeMin
		}

		if res.MsgTimeMax > totals.MsgTimeMax {
			totals.MsgTimeMax = res.MsgTimeMax
		}

		msgTimeMeans[i] = res.MsgTimeMean
		msgsPerSecs[i] = res.MsgsPerSec
		runTimes[i] = res.RunTime
		bws[i] = res.MsgsPerSec
	}
	totals.AvgMsgsPerSec = stats.StatsMean(msgsPerSecs)
	totals.AvgRunTime = stats.StatsMean(runTimes)
	totals.MsgTimeMeanAvg = stats.StatsMean(msgTimeMeans)
	// calculate std if sample is > 1, otherwise leave as 0 (convention)
	if sampleSize > 1 {
		totals.MsgTimeMeanStd = stats.StatsSampleStandardDeviation(msgTimeMeans)
	}

	return totals
}

func printResults(results []*RunResults, totals *TotalResults, format string) {
	switch format {
	case "json":
		jr := JSONResults{
			Runs:   results,
			Totals: totals,
		}
		data, err := json.Marshal(jr)
		if err != nil {
			log.Fatalf("Error marshalling results: %v", err)
		}
		var out bytes.Buffer
		_ = json.Indent(&out, data, "", "\t")

		fmt.Println(out.String())
	default:
		for _, res := range results {
			fmt.Printf("======= CLIENT %d =======\n", res.ID)
			fmt.Printf("Number of messages received: %d\n", res.Successes)
			fmt.Printf("Runtime (s):                 %.3f\n", res.RunTime)
			fmt.Printf("Msg latency min (ms):        %.3f\n", res.MsgTimeMin / 1_000_000)
			fmt.Printf("Msg latency max (ms):        %.3f\n", res.MsgTimeMax / 1_000_000)
			fmt.Printf("Msg latency mean (ms):       %.3f\n", res.MsgTimeMean / 1_000_000)
			fmt.Printf("Msg latency std (ms):        %.3f\n", res.MsgTimeStd / 1_000_000)
			fmt.Printf("Bandwidth (msg/sec):         %.3f\n", res.MsgsPerSec)
			fmt.Printf("Duplicates:                  %d\n\n", res.Duplicates)
		}
		fmt.Printf("========= TOTAL (%d) =========\n", len(results))
		fmt.Printf("Number of messages received: %d\n", totals.Successes)
		fmt.Printf("Total Runtime (sec):         %.3f\n", totals.TotalRunTime)
		fmt.Printf("Average Runtime (sec):       %.3f\n", totals.AvgRunTime)
		fmt.Printf("Msg latency min (ms):        %.3f\n", totals.MsgTimeMin / 1_000_000)
		fmt.Printf("Msg latency max (ms):        %.3f\n", totals.MsgTimeMax / 1_000_000)
		fmt.Printf("Msg latency mean mean (ms):  %.3f\n", totals.MsgTimeMeanAvg / 1_000_000)
		fmt.Printf("Msg latency mean std (ms):   %.3f\n", totals.MsgTimeMeanStd / 1_000_000)
		fmt.Printf("Average Bandwidth (msg/sec): %.3f\n", totals.AvgMsgsPerSec)
		fmt.Printf("Total Bandwidth (msg/sec):   %.3f\n", totals.TotalMsgsPerSec)
		fmt.Printf("Duplicates:                  %d\n\n", totals.Duplicates)
	}
}

func generateTLSConfig(certFile string, keyFile string) *tls.Config {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Error reading certificate files: %v", err)
	}

	cfg := tls.Config{
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	return &cfg
}
