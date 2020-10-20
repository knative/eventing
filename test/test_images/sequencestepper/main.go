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
	"log"

	"knative.dev/eventing/pkg/kncloudevents"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var (
	eventMsgAppender string
)

func init() {
	flag.StringVar(&eventMsgAppender, "msg-appender", "", "a string we want to append on the event message")
}

func gotEvent(event cloudevents.Event) (*cloudevents.Event, error) {
	log.Println("Received a new event: ")
	log.Printf("[%s] %s %s: %s", event.Time().String(), event.Source(), event.Type(), string(event.Data()))

	outputEvent := event.Clone()

	// append eventMsgAppender to message of the data
	var data map[string]interface{}
	if err := json.Unmarshal(event.Data(), &data); err != nil {
		return nil, err
	}
	data["msg"] = data["msg"].(string) + eventMsgAppender
	if eventData, err := json.Marshal(&data); err != nil {
		return nil, err
	} else if err := outputEvent.SetData(cloudevents.ApplicationJSON, eventData); err != nil {
		return nil, err
	}

	log.Println("Transform the event to: ")
	log.Printf("[%s] %s %s: %s", outputEvent.Time().String(), outputEvent.Source(), outputEvent.Type(), string(outputEvent.Data()))

	return &outputEvent, nil
}

func main() {
	// parse the command line flags
	flag.Parse()

	t, err := cloudevents.NewHTTP(
		cloudevents.WithPort(8080),
		cloudevents.WithMiddleware(kncloudevents.CreateHandler),
	)
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}

	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("listening on 8080")
	if err := c.StartReceiver(context.Background(), gotEvent); err != nil {
		log.Fatalf("failed to start receiver: %s", err)
	}
}
