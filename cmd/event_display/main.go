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
	"os"

	"github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"

	"go.uber.org/zap"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
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
	c, err := client.NewClientHTTP(
		[]cehttp.Option{cehttp.WithMiddleware(healthzMiddleware)}, nil,
	)
	if err != nil {
		log.Fatal("Failed to create client: ", err)
	}
	conf, err := config.JSONToTracingConfig(os.Getenv("K_CONFIG_TRACING"))
	if err != nil {
		log.Printf("Failed to read tracing config, using the no-op default: %v", err)
	}
	if err := tracing.SetupStaticPublishing(zap.L().Sugar(), "", conf); err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}

	if err := c.StartReceiver(ctx, display); err != nil {
		log.Fatal("Error during receiver's runtime: ", err)
	}
}

// HTTP path of the health endpoint used for probing the service.
const healthzPath = "/healthz"

// healthzMiddleware is a cehttp.Middleware which exposes a health endpoint.
func healthzMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.RequestURI == healthzPath {
			w.WriteHeader(http.StatusNoContent)
		} else {
			next.ServeHTTP(w, req)
		}
	})
}
