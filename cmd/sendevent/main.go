/*
Copyright 2018 Google LLC

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
// Not wired to bazel because this utility is meant to be run directly rather
// than deployed to K8S
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/elafros/eventing/pkg/event"
)

var (
	context event.Context
	webhook string
	data    string
)

func init() {
	flag.StringVar(&context.EventID, "event-id", "", "Event ID to use. Defaults to a UUID")
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
	req, err := event.NewRequest(webhook, untyped, context)
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

func fillEventContext(ctx *event.Context) {
	ctx.CloudEventsVersion = "0.1"
	ctx.EventTime = time.Now().UTC()

	if ctx.EventID == "" {
		// A "UUID". Not technically spec complaint
		b := make([]byte, 16)
		if n, err := rand.Read(b); n != 16 || err != nil {
			fmt.Println("Could not create event-id UUID")
			os.Exit(1)
		}
		ctx.EventID = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}

	if ctx.Source == "" {
		var err error
		ctx.Source, err = os.Hostname()
		if err != nil {
			ctx.Source = "localhost"
		}
	}
}
