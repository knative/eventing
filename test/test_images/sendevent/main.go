/*
Copyright 2019 The Knative Authors

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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"log"
	"os"
	"strconv"
	"time"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Data     string `json:"data"`
}

var (
	sink      string
	data      string
	eventID   string
	eventType string
	source    string
	periodStr string
	delayStr  string
	maxMsgStr string
	encoding  string
)

func init() {
	flag.StringVar(&sink, "sink", "", "The sink url for the message destination.")
	flag.StringVar(&data, "data", `{"hello": "world!"}`, "Cloudevent data body.")
	flag.StringVar(&eventID, "event-id", "", "Event ID to use. Defaults to a generated UUID")
	flag.StringVar(&eventType, "event-type", "knative.eventing.test.e2e", "The Event Type to use.")
	flag.StringVar(&source, "source", "", "Source URI to use. Defaults to the current machine's hostname")
	flag.StringVar(&periodStr, "period", "5", "The number of seconds between messages.")
	flag.StringVar(&delayStr, "delay", "5", "The number of seconds to wait before sending messages.")
	flag.StringVar(&maxMsgStr, "max-messages", "1", "The number of messages to attempt to send. 0 for unlimited.")
	flag.StringVar(&encoding, "encoding", "binary", "The encoding of the cloud event, one of(binary, structured).")
}

func parseDurationStr(durationStr string, defaultDuration int) time.Duration {
	var duration time.Duration
	if d, err := strconv.Atoi(durationStr); err != nil {
		duration = time.Duration(defaultDuration) * time.Second
	} else {
		duration = time.Duration(d) * time.Second
	}
	return duration
}

func main() {
	flag.Parse()
	period := parseDurationStr(periodStr, 5)
	delay := parseDurationStr(delayStr, 5)

	maxMsg := 1
	if m, err := strconv.Atoi(maxMsgStr); err == nil {
		maxMsg = m
	}

	if delay > 0 {
		log.Printf("will sleep for %s", delay)
		time.Sleep(delay)
		log.Printf("awake, contining")
	}

	if source == "" {
		source = "localhost"
	}

	var encodingOption http.Option
	switch encoding {
	case "binary":
		encodingOption = http.WithBinaryEncoding()
	case "structured":
		encodingOption = http.WithStructuredEncoding()
	default:
		fmt.Printf("unsupported encoding option: %q\n", encoding)
		os.Exit(1)
	}

	t, err := http.New(
		http.WithTarget(sink),
		encodingOption,
	)
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}
	c, err := client.New(t,
		client.WithTimeNow(),
		client.WithUUIDs(),
	)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	var untyped map[string]interface{}
	if err := json.Unmarshal([]byte(data), &untyped); err != nil {
		fmt.Println("Currently sendevent only supports JSON event data")
		os.Exit(1)
	}

	sequence := 0

	ticker := time.NewTicker(period)
	for {
		sequence++
		untyped["sequence"] = fmt.Sprintf("%d", sequence)

		event := cloudevents.Event{
			Context: cloudevents.EventContextV02{
				ID:     eventID,
				Type:   eventType,
				Source: *types.ParseURLRef(source),
			}.AsV02(),
			Data: untyped,
		}

		if resp, err := c.Send(context.Background(), event); err != nil {
			fmt.Printf("send returned an error: %v\n", err)
		} else if resp != nil {
			fmt.Printf("Got response from %s\n%s\n", sink, resp)
		}

		// Wait for next tick
		<-ticker.C
		// Only send a limited number of messages.
		if maxMsg != 0 && maxMsg == sequence {
			return
		}
	}
}
