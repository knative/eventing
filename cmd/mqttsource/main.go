/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	sink   string
	source string

	// CloudEvents specific parameters
	eventType   string
	eventSource string

	// MQTT specific parameters
	broker   string
	clientID string
	topic    string
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to send messages to")
	flag.StringVar(&source, "source", "", "the url to get messages from")
	flag.StringVar(&eventType, "eventType", "mqtt-event", "the event-type (CloudEvents)")
	flag.StringVar(&eventSource, "eventSource", "", "the event-source (CloudEvents)")

	// MQTT parameters
	flag.StringVar(&broker, "broker", "tcp://mqtt.example.com:1883", "the MQTT broker address")
	flag.StringVar(&clientID, "clientID", "mqtt-source", "the MQTT client ID")
	flag.StringVar(&topic, "topic", "mqtt-topic", "the MQTT topic to subscribe to")
}

func main() {
	flag.Parse()

	k_sink := os.Getenv("K_SINK")
	if k_sink != "" {
		sink = k_sink
	}

	// "source" flag must not be empty for operation.
	if source == "" {
		log.Fatal("A valid source url must be defined.")
	}

	// The event's source defaults to the URL of where it was taken from.
	if eventSource == "" {
		eventSource = source
	}

	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
	}

	ce, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("Failed to create a http cloudevent client: %s", err.Error())
	}

	ctx := cloudevents.ContextWithTarget(context.Background(), sink)

	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		event := cloudevents.NewEvent(cloudevents.VersionV1)
		event.SetType(eventType)
		event.SetSource(eventSource)
		event.SetID(fmt.Sprintf("%d", time.Now().UnixNano())) // Set a unique ID for each event
		_ = event.SetData(cloudevents.ApplicationJSON, msg.Payload())
		if result := ce.Send(ctx, event); !cloudevents.IsACK(result) {
			log.Printf("sending event to channel failed: %v", result)
		}
	})

	select {} // Keep the program running indefinitely
}
