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
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Label    string `json:"label"`
}

func receive(event cloudevents.Event) {
	hb := &Heartbeat{}
	if err := event.DataAs(hb); err != nil {
		fmt.Printf("got data error: %s\n", err.Error())
	}
	log.Printf("CloudEvent:\n%s", event)
	log.Printf("[%s] %s %s: ", event.Time(), event.DataContentType(), event.Source())
	log.Printf("\tSequence number: %d, label: %q", hb.Sequence, hb.Label)
}

func main() {
	ctx := context.TODO()

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	if err := c.StartReceiver(ctx, receive); err != nil {
		log.Fatalf("failed to start receiver: %s", err.Error())
	}

	log.Printf("listening on port %d\n", 8080)
	<-ctx.Done()
}
