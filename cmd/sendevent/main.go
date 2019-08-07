/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Implements a simple utility for sending a JSON-encoded sample event.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go"
	"knative.dev/eventing/pkg/utils"
)

var (
	eventID   string
	eventType string
	source    string
	data      string
)

func init() {
	flag.StringVar(&eventID, "event-id", "", "Event ID to use. Defaults to a generated UUID")
	flag.StringVar(&eventType, "event-type", "google.events.action.demo", "The Event Type to use.")
	flag.StringVar(&source, "source", "", "Source URI to use. Defaults to the current machine's hostname")
	flag.StringVar(&data, "data", `{"hello": "world!"}`, "Event data")
}

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Println("Usage: sendevent [flags] <webhook>\nFor details about valid flags, run sendevent --help")
		os.Exit(1)
	}

	target := flag.Arg(0)

	var untyped map[string]interface{}
	if err := json.Unmarshal([]byte(data), &untyped); err != nil {
		fmt.Println("Currently sendevent only supports JSON event data")
		os.Exit(1)
	}

	if source == "" {
		source = fmt.Sprintf("http://%s", utils.GetClusterDomainName())
	}

	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(target),
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		log.Printf("failed to create transport, %v", err)
		os.Exit(1)
	}
	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		log.Printf("failed to create client, %v", err)
		os.Exit(1)
	}

	event := cloudevents.NewEvent()
	if eventID != "" {
		event.SetID(eventID)
	}
	event.SetType(eventType)
	event.SetSource(source)
	if err := event.SetData(untyped); err != nil {
		log.Printf("failed to set data, %v", err)
		os.Exit(1)
	}

	if resp, err := c.Send(context.Background(), event); err != nil {
		fmt.Printf("Failed to send event to %s: %s\n", target, err)
		os.Exit(1)
	} else if resp != nil {
		fmt.Printf("Got response from %s\n%s\n", target, resp)
	}
}
