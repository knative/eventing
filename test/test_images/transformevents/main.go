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
	"fmt"
	"log"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
)

type example struct {
	ID      string `json:"sequence"`
	Message string `json:"msg"`
}

var (
	msgPostfix string
)

func init() {
	flag.StringVar(&msgPostfix, "msg-postfix", "", "The postfix we want to add for the message.")
}

func gotEvent(event cloudevents.Event, resp *cloudevents.EventResponse) error {
	ctx := event.Context.AsV02()

	data := &example{}
	if err := event.DataAs(data); err != nil {
		fmt.Printf("Got Data Error: %s\n", err.Error())
		return err
	}

	data.Message += msgPostfix
	log.Printf("[%s] %s %s: %+v", ctx.Time.String(), *ctx.ContentType, ctx.Source.String(), data)

	r := cloudevents.Event{
		Context: event.Context,
		Data:    data,
	}
	resp.RespondWith(200, &r)
	return nil
}

func main() {
	// parse the command line flags
	flag.Parse()
	c, err := client.NewDefault()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("listening on 8080")
	log.Printf("msgPostfix: %s", msgPostfix)
	log.Fatalf("failed to start receiver: %s", c.StartReceiver(context.Background(), gotEvent))
}
