package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"
	"encoding/json"

	"github.com/GaryBoone/GoStats/stats"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client implements an MQTT client running benchmark test
type Client struct {
	ID              int
	ClientID        string
	BrokerURL       string
	BrokerUser      string
	BrokerPass      string
	MsgTopic        string
	ReceiveCount    int64
	MsgQoS          byte
	Quiet           bool
	WaitTimeout time.Duration
	TLSConfig   *tls.Config
}

// Run runs benchmark tests and writes results in the provided channel
func (c *Client) Run(res chan *RunResults) {
	received := make(chan *Message)
	runResults := new(RunResults)

	var started *time.Time = nil
	// start subscriber
	go c.receiveMessages(received)

	runResults.ID = c.ID

	receivedMessages := make([]*Message, c.ReceiveCount)
	var receivedSoFar int64 = 0
	for {
        m := <-received
        // Capture message
        if receivedSoFar < c.ReceiveCount {
            receivedMessages[receivedSoFar] = m
        } else {
            log.Printf("CLIENT %v received too many messages (probably duplicates): %v\n", c.ID, m)
        }
        // Count all received messages
        receivedSoFar++

        // Start counting from the first received message
        if started == nil {
            var now = time.Now()
            started = &now
        }

        // Print progress every so often
        if receivedSoFar % 100 == 0 {
            log.Printf("CLIENT %v Received %d of messages out of %d\n", c.ID, receivedSoFar, c.ReceiveCount)
        }

        // Check if we are done
        if receivedSoFar >= c.ReceiveCount {
            latencies := make([]float64, c.ReceiveCount)
            for i, message := range receivedMessages {
                latencies[i] = float64(message.ReceivedAt - message.Payload.GeneratedAt) // in nanoseconds
            }
            // calculate results
            runResults.Successes = int64(len(receivedMessages))
            duration := time.Since(*started)
            runResults.MsgTimeMin = stats.StatsMin(latencies)
            runResults.MsgTimeMax = stats.StatsMax(latencies)
            runResults.MsgTimeMean = stats.StatsMean(latencies)
            runResults.RunTime = duration.Seconds()
            runResults.MsgsPerSec = float64(runResults.Successes) / duration.Seconds()
            runResults.Duplicates = receivedSoFar - c.ReceiveCount
            // calculate std if sample is > 1, otherwise leave as 0 (convention)
            if c.ReceiveCount > 1 {
                runResults.MsgTimeStd = stats.StatsSampleStandardDeviation(latencies)
            }

            // report results and exit
            res <- runResults
            return
        }
	}
}

func (c *Client) receiveMessages(received chan *Message) {
	onConnected := func(client mqtt.Client) {
		if !c.Quiet {
			log.Printf("CLIENT %v is connected to the broker %v\n", c.ID, c.BrokerURL)
		}
	}

	onMessage := func(client mqtt.Client, msg mqtt.Message) {
	    var payload Payload
	    err := json.Unmarshal(msg.Payload(), &payload)

	    if err != nil {
	        log.Printf("CLIENT %v received message which could not be unmarshalled from JSON: %v\n", c.ID, err)
	    } else {
	        received <- &Message {
	            Payload: payload,
	            ReceivedAt: time.Now().UnixNano(),
	        }
	    }
	}

	opts := mqtt.NewClientOptions().
		AddBroker(c.BrokerURL).
		SetClientID(fmt.Sprintf("Subscriber-%s-%v", c.ClientID, c.ID)).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetOnConnectHandler(onConnected).
		SetConnectionLostHandler(func(client mqtt.Client, reason error) {
			log.Printf("CLIENT %v lost connection to the broker: %v. Will reconnect...\n", c.ID, reason.Error())
		}).
		SetDefaultPublishHandler(onMessage)
	if c.BrokerUser != "" && c.BrokerPass != "" {
		opts.SetUsername(c.BrokerUser)
		opts.SetPassword(c.BrokerPass)
	}
	if c.TLSConfig != nil {
		opts.SetTLSConfig(c.TLSConfig)
	}

	client := mqtt.NewClient(opts)
	connectToken := client.Connect()
	connectToken.Wait()
	if connectToken.Error() != nil {
        log.Printf("CLIENT %v had error connecting to the broker: %v\n", c.ID, connectToken.Error())
    }
	subscribetoken := client.Subscribe(c.MsgTopic, c.MsgQoS, nil)
	subscribetoken.Wait()

	if subscribetoken.Error() != nil {
        log.Printf("CLIENT %v had error subscribing to the broker: %v\n", c.ID, subscribetoken.Error())
    }
}
