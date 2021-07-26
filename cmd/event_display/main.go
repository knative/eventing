/*
Copyright 2021 The Knative Authors

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
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

/*
Example Output:

☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.eventing.samples.heartbeat
  source: https://knative.dev/eventing-contrib/cmd/heartbeats/#event-test/mypod
  id: 2b72d7bf-c38f-4a98-a433-608fbcdd2596
  time: 2019-10-18T15:23:20.809775386Z
  contenttype: application/json
Extensions,
  beats: true
  heart: yes
  the: 42
Data,
  {
    "id": 2,
    "label": ""
  }
*/

// display prints the given Event in a human-readable format.
func display(event cloudevents.Event) {
	fmt.Printf("☁️  cloudevents.Event\n%s", event)
}

func main() {
	run(context.Background())
}

func run(ctx context.Context) {
	c, err := cloudevents.NewClientHTTP(
		cehttp.WithMiddleware(healthzMiddleware()),
	)
	if err != nil {
		log.Fatal("Failed to create client: ", err)
	}

	if err := c.StartReceiver(ctx, display); err != nil {
		log.Fatal("Error during receiver's runtime: ", err)
	}
}

// HTTP path of the health endpoint used for probing the service.
const healthzPath = "/healthz"

// healthzMiddleware returns a cehttp.Middleware which exposes a health
// endpoint by registering a handler in the multiplexer of the CloudEvents HTTP
// client.
func healthzMiddleware() cehttp.Middleware {
	return func(next http.Handler) http.Handler {
		next.(*http.ServeMux).Handle(healthzPath, http.HandlerFunc(handleHealthz))
		return next
	}
}

// handleHealthz is a http.Handler which responds to health requests.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
