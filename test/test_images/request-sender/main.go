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
	"flag"
	"io"
	"log"
	nethttp "net/http"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/test/lib/sender"
	"knative.dev/eventing/test/test_images"
)

var (
	sink         string
	method       string
	responseSink string
	inputHeaders string
	inputBody    string
	periodStr    string
	delayStr     string
	maxMsg       int
	addTracing   bool
)

func init() {
	flag.StringVar(&sink, "sink", "", "The sink url for the message destination.")
	flag.StringVar(&method, "method", "POST", "HTTP Method to send.")
	flag.StringVar(&responseSink, "response-sink", "", "The response sink url to send the response.")
	flag.StringVar(&inputHeaders, "headers", "", "HTTP Headers to send.")
	flag.StringVar(&inputBody, "body", "", "HTTP body to send.")
	flag.StringVar(&periodStr, "period", "5", "The number of seconds between messages.")
	flag.StringVar(&delayStr, "delay", "5", "The number of seconds to wait before sending messages.")
	flag.IntVar(&maxMsg, "max-messages", 1, "The number of messages to attempt to send. 0 for unlimited.")
	flag.BoolVar(&addTracing, "add-tracing", false, "Should tracing be added to events sent.")
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

	// I need the httpClient to report to responseSink
	ceClient, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("failed to create httpClient, %v", err)
	}

	httpClient := &nethttp.Client{
		Transport: &ochttp.Transport{
			Base:        nethttp.DefaultTransport,
			Propagation: tracecontextb3.TraceContextEgress,
		},
	}

	sequence := 0

	ticker := time.NewTicker(period)
	for {
		sequence++

		var body io.Reader
		if inputBody != "" {
			body = strings.NewReader(inputBody)
			log.Printf("Using body: '%s'", inputBody)
		}

		request, err := nethttp.NewRequest(method, sink, body)
		if err != nil {
			log.Fatalf("Cannot create request: %s", err.Error())
		}

		if inputHeaders != "" {
			for k, v := range test_images.ParseHeaders(inputHeaders) {
				request.Header.Set(k, v[0])
				log.Printf("Using header %s: %s", k, v[0])
			}
		}

		response, err := httpClient.Do(request)
		if err != nil {
			log.Fatalf("Error while executing HTTP request: %v", err.Error())
		}

		responseEvent := sender.NewSenderEventFromRaw(
			uuid.New().String(),
			"https://knative.dev/eventing/test/event-sender",
			response,
		)

		if responseSink != "" {
			res := ceClient.Send(cloudevents.ContextWithTarget(context.Background(), responseSink), responseEvent)
			if cloudevents.IsUndelivered(res) {
				log.Printf("send to response sink returned an error: %v\n", res)
			} else {
				log.Printf("Got response from %s\n%s\n", responseSink, res)
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
