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

	ce "github.com/cloudevents/sdk-go/v1"

	"knative.dev/eventing/test/lib/cloudevents"
)

var (
	eventMsgAppender string
)

func init() {
	flag.StringVar(&eventMsgAppender, "msg-appender", "", "a string we want to append on the event message")
}

func gotEvent(event ce.Event, resp *ce.EventResponse) error {
	ctx := event.Context.AsV1()

	data := &cloudevents.BaseData{}
	if err := event.DataAs(data); err != nil {
		log.Printf("Got Data Error: %s\n", err.Error())
		return err
	}

	log.Println("Received a new event: ")
	log.Printf("[%s] %s %s: %+v", ctx.Time.String(), ctx.GetSource(), ctx.GetType(), data)

	// append eventMsgAppender to message of the data
	data.Message = data.Message + eventMsgAppender
	r := ce.Event{
		Context: ctx,
		Data:    data,
	}

	r.SetDataContentType(ce.ApplicationJSON)

	log.Println("Transform the event to: ")
	log.Printf("[%s] %s %s: %+v", ctx.Time.String(), ctx.GetSource(), ctx.GetType(), data)

	resp.RespondWith(200, &r)
	return nil
}

func main() {
	// parse the command line flags
	flag.Parse()

	c, err := ce.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("listening on 8080")
	log.Fatalf("failed to start receiver: %s", c.StartReceiver(context.Background(), gotEvent))
}
