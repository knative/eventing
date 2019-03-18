/*
Copyright 2018 The Knative Authors

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

// Implements a simple utility for sending a JSON-encoded sample event.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/knative/pkg/cloudevents"

	"github.com/google/uuid"
)

var (
	context cloudevents.EventContext
	webhook string
	data    string
)

func init() {
	flag.StringVar(&context.EventID, "event-id", "", "Event ID to use. Defaults to a generated UUID")
	flag.StringVar(&context.EventType, "event-type", "google.events.action.demo", "The Event Type to use.")
	flag.StringVar(&context.Source, "source", "", "Source URI to use. Defaults to the current machine's hostname")
	flag.StringVar(&data, "data", `{"hello": "world!"}`, "Event data")
}

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Println("Usage: sendevent [flags] <webhook>\nFor details about valid flags, run sendevent --help")
		os.Exit(1)
	}

	webhook := flag.Arg(0)

	var untyped map[string]interface{}
	if err := json.Unmarshal([]byte(data), &untyped); err != nil {
		fmt.Println("Currently sendevent only supports JSON event data")
		os.Exit(1)
	}

	fillEventContext(&context)
	req, err := cloudevents.NewRequest(webhook, untyped, context)
	if err != nil {
		fmt.Printf("Failed to create request: %s", err)
		os.Exit(1)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Failed to send event to %s: %s\n", webhook, err)
		os.Exit(1)
	}
	fmt.Printf("Got response from %s\n%s\n", webhook, res.Status)
	if res.Header.Get("Content-Length") != "" {
		bytes, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(bytes))
	}
}

func fillEventContext(ctx *cloudevents.EventContext) {
	ctx.CloudEventsVersion = "0.1"
	ctx.EventTime = time.Now().UTC()

	if ctx.EventID == "" {
		ctx.EventID = uuid.New().String()
	}

	if ctx.Source == "" {
		var err error
		ctx.Source, err = os.Hostname()
		if err != nil {
			ctx.Source = "localhost"
		}
	}
}
