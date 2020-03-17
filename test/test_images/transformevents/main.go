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

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
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

func gotEvent(event cloudevents.Event, resp *cloudevents.EventResponse) error {
	ctx := event.Context.AsV1()

	dataBytes, err := event.DataBytes()
	if err != nil {
		log.Printf("Got Data Error: %s\n", err.Error())
		return err
	}
	log.Println("Received a new event: ")
	log.Printf("[%s] %s %s: %s", ctx.Time.String(), ctx.GetSource(), ctx.GetType(), dataBytes)

	if eventSource != "" {
		ctx.SetSource(eventSource)
	}
	if eventType != "" {
		ctx.SetType(eventType)
	}
	if eventData != "" {
		dataBytes = []byte(eventData)
	}
	r := cloudevents.Event{
		Context: ctx,
		Data:    string(dataBytes),
	}
	r.SetDataContentType(cloudevents.ApplicationJSON)

	log.Println("Transform the event to: ")
	log.Printf("[%s] %s %s: %s", ctx.Time.String(), ctx.GetSource(), ctx.GetType(), dataBytes)

	resp.RespondWith(200, &r)
	return nil
}

func main() {
	// parse the command line flags
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracing.AlwaysSample); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	c, err := kncloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("listening on 8080")
	log.Fatalf("failed to start receiver: %s", c.StartReceiver(context.Background(), gotEvent))
}
