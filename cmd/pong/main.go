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

// Implements a simple utility function to demonstrate responses.
package main

import (
	"context"
	"flag"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"github.com/google/uuid"
)

var id = uuid.New().String()

var (
	pingType string
	pongType string
)

func init() {
	flag.StringVar(&pingType, "ping", "dev.knative.ping", "Watches for this CloudEvent Type.")
	flag.StringVar(&pongType, "pong", "dev.knative.pong", "Responds with this CloudEvent Type.")
}

func receive(event cloudevents.Event, resp *cloudevents.EventResponse) {
	log.Printf("Received CloudEvent,\n%s", event)
	if event.Type() == pingType {
		pong := cloudevents.NewEvent()
		pong.SetType(pongType)
		pong.SetSource("knative.dev/eventing/cmd/pong/" + id)
		if err := pong.SetData(event.Data); err != nil {
			log.Printf("failed to set data on pong: %s", err)
			resp.Error(400, "bad data")
			return
		}
		resp.RespondWith(200, &pong)
	}
}

func main() {
	flag.Parse()

	ce, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create CloudEvent client, %s", err)
	}

	log.Fatal(ce.StartReceiver(context.Background(), receive))
}
