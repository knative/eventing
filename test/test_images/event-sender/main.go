/*
Copyright 2020 The Knative Authors

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
	"log"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/tracing"
)

var (
	sink          string
	inputEvent    string
	eventEncoding string
	periodStr     string
	delayStr      string
	maxMsgStr     string
	addTracing    bool
	incrementalId bool
)

func init() {
	flag.StringVar(&sink, "sink", "", "The sink url for the message destination.")
	flag.StringVar(&inputEvent, "event", "", "Event JSON encoded")
	flag.StringVar(&eventEncoding, "event-encoding", "binary", "The encoding of the cloud event: [binary, structured].")
	flag.StringVar(&periodStr, "period", "5", "The number of seconds between messages.")
	flag.StringVar(&delayStr, "delay", "5", "The number of seconds to wait before sending messages.")
	flag.StringVar(&maxMsgStr, "max-messages", "1", "The number of messages to attempt to send. 0 for unlimited.")
	flag.BoolVar(&addTracing, "add-tracing", false, "Should tracing be added to events sent.")
	flag.BoolVar(&incrementalId, "incremental-id", false, "Override the event id with an incremental id.")
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

	defer func() {
		var err error
		r := recover()
		if r != nil {
			err = r.(error)
			log.Printf("recovered from panic: %v", err)
		}
	}()

	if delay > 0 {
		log.Printf("will sleep for %s", delay)
		time.Sleep(delay)
		log.Printf("awake, continuing")
	}

	ctx := context.Background()
	switch eventEncoding {
	case "binary":
		ctx = cloudevents.WithEncodingBinary(ctx)
	case "structured":
		ctx = cloudevents.WithEncodingStructured(ctx)
	default:
		log.Fatalf("unsupported encoding option: %q\n", eventEncoding)
	}

	t, err := cloudevents.NewHTTP(cloudevents.WithTarget(sink))
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}

	if addTracing {
		log.Println("Adding tracing")
		logger, _ := zap.NewDevelopment()
		if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracing.AlwaysSample); err != nil {
			log.Fatalf("Unable to setup trace publishing: %v", err)
		}
	}

	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	var baseEvent cloudevents.Event
	if err := json.Unmarshal([]byte(inputEvent), &baseEvent); err != nil {
		log.Fatalf("Unable to unmarshal the event from json: %v", err)
	}

	sequence := 0

	ticker := time.NewTicker(period)
	for {
		event := baseEvent.Clone()

		sequence++
		event.SetExtension("sequence", sequence)

		if incrementalId {
			event.SetID(fmt.Sprintf("%d", sequence))
		}

		if responseEvent, result := c.Request(ctx, event); !cloudevents.IsACK(result) {
			log.Printf("send returned an error: %v\n", result)
		} else if responseEvent != nil {
			log.Printf("Got response from %s\n%s\n", sink, *responseEvent)
		}

		// Wait for next tick
		<-ticker.C
		// Only send a limited number of messages.
		if maxMsg != 0 && maxMsg == sequence {
			return
		}
	}
}
