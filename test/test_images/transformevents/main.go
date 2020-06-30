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
	"flag"
	"log"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/test/test_images"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
)

var (
	eventType   string
	eventSource string
	eventData   string
)

func init() {
	flag.StringVar(&eventType, "event-type", "", "The Event Type to use.")
	flag.StringVar(&eventSource, "event-source", "", "Source URI to use. Defaults to the current machine's hostname")
	flag.StringVar(&eventData, "event-data", "", "Cloudevent data body.")
}

func gotEvent(event cloudevents.Event) (*cloudevents.Event, error) {
	log.Println("Received a new event: ")
	log.Printf("[%s] %s %s: %s", event.Time().String(), event.Source(), event.Type(), string(event.Data()))

	outputEvent := event.Clone()

	if eventSource != "" {
		outputEvent.SetSource(eventSource)
	}
	if eventType != "" {
		outputEvent.SetType(eventType)
	}
	if eventData != "" {
		if err := outputEvent.SetData(cloudevents.ApplicationJSON, []byte(eventData)); err != nil {
			return nil, err
		}
	}

	log.Println("Transform the event to: ")
	log.Printf("[%s] %s %s: %s", outputEvent.Time().String(), outputEvent.Source(), outputEvent.Type(), string(outputEvent.Data()))

	return &outputEvent, nil
}

func main() {
	// parse the command line flags
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	if err := test_images.ConfigureTracing(logger.Sugar(), ""); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	t, err := cloudevents.NewHTTP(
		cloudevents.WithPort(8080),
		cloudevents.WithMiddleware(kncloudevents.CreateHandler),
	)
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}

	c, err := cloudevents.NewClientObserved(t,
		cloudevents.WithTracePropagation,
	)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("listening on 8080")
	if err := c.StartReceiver(context.Background(), gotEvent); err != nil {
		log.Fatalf("failed to start receiver: %s", err)
	}
}
