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
	nethttp "net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/plugin/ochttp"
	"go.uber.org/zap"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/test/lib/sender"
	"knative.dev/eventing/test/test_images"
)

var (
	sink              string
	responseSink      string
	inputEvent        string
	eventEncoding     string
	periodStr         string
	delayStr          string
	maxMsg            int
	addTracing        bool
	addSequence       bool
	incrementalId     bool
	additionalHeaders string
)

func init() {
	flag.StringVar(&sink, "sink", "", "The sink url for the message destination.")
	flag.StringVar(&responseSink, "response-sink", "", "The response sink url to send the response.")
	flag.StringVar(&inputEvent, "event", "", "Event JSON encoded")
	flag.StringVar(&eventEncoding, "event-encoding", "binary", "The encoding of the cloud event: [binary, structured].")
	flag.StringVar(&periodStr, "period", "5", "The number of seconds between messages.")
	flag.StringVar(&delayStr, "delay", "5", "The number of seconds to wait before sending messages.")
	flag.IntVar(&maxMsg, "max-messages", 1, "The number of messages to attempt to send. 0 for unlimited.")
	flag.BoolVar(&addTracing, "add-tracing", false, "Should tracing be added to events sent.")
	flag.BoolVar(&addSequence, "add-sequence-extension", false, "Should add extension 'sequence' identifying the sequence number.")
	flag.BoolVar(&incrementalId, "incremental-id", false, "Override the event id with an incremental id.")
	flag.StringVar(&additionalHeaders, "additional-headers", "", "Additional non-CloudEvents headers to send")
}

func main() {
	flag.Parse()
	period := test_images.ParseDurationStr(periodStr, 5)
	delay := test_images.ParseDurationStr(delayStr, 5)

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

	httpOpts := []cehttp.Option{
		cloudevents.WithTarget(sink),
	}

	if addTracing {
		httpOpts = append(
			httpOpts,
			cloudevents.WithRoundTripper(&ochttp.Transport{
				Base:        nethttp.DefaultTransport,
				Propagation: tracecontextb3.TraceContextEgress,
			}),
		)
	}
	if additionalHeaders != "" {
		for k, v := range test_images.ParseHeaders(additionalHeaders) {
			httpOpts = append(httpOpts, cloudevents.WithHeader(k, v[0]))
		}
	}

	t, err := cloudevents.NewHTTP(httpOpts...)
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}

	var c cloudevents.Client
	if addTracing {
		log.Println("Adding tracing")
		logger, _ := zap.NewDevelopment()
		if err := test_images.ConfigureTracing(logger.Sugar(), ""); err != nil {
			log.Fatalf("Unable to setup trace publishing: %v", err)
		}

		c, err = cloudevents.NewClientObserved(t,
			cloudevents.WithTracePropagation,
		)
	} else {
		c, err = cloudevents.NewClient(t)
	}

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
		if addSequence {
			event.SetExtension("sequence", sequence)
		}
		if incrementalId {
			event.SetID(fmt.Sprintf("%d", sequence))
		}

		log.Printf("I'm going to send\n%s\n", event)

		responseEvent, responseResult := c.Request(ctx, event)
		if cloudevents.IsUndelivered(responseResult) {
			log.Printf("send returned an error: %v\n", responseResult)
		} else {
			if responseEvent != nil {
				log.Printf("Got response from %s\n%s\n%s\n", sink, responseResult, *responseEvent)
			} else {
				log.Printf("Got response from %s\n%s\n", sink, responseResult)
			}

			if responseSink != "" {
				var httpResult *cehttp.Result
				cloudevents.ResultAs(responseResult, &httpResult)
				responseEvent := sender.NewSenderEvent(
					event.ID(),
					"https://knative.dev/eventing/test/event-sender",
					responseEvent,
					httpResult,
				)

				result2 := c.Send(cloudevents.ContextWithTarget(context.Background(), responseSink), responseEvent)
				if cloudevents.IsUndelivered(result2) {
					log.Printf("send to response sink returned an error: %v\n", result2)
				} else {
					log.Printf("Got response from %s\n%s\n", responseSink, result2)
				}
			}
		}

		// Wait for next tick
		<-ticker.C
		// Only send a limited number of messages.
		if maxMsg != 0 && maxMsg == sequence {
			return
		}
	}
}
