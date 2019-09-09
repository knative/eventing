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
	"log"
	"os"

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/tracing"

	cloudevents "github.com/cloudevents/sdk-go"
	"knative.dev/eventing/pkg/kncloudevents"
)

func handler(event cloudevents.Event) {
	if err := event.Validate(); err == nil {
		log.Printf("%s", event.Data.([]byte))
	} else {
		log.Printf("error validating the event: %v", err)
	}
}

func main() {
	name, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get hostname: %v", err)
	}
	if err := tracing.SetupStaticPublishing(zap.NewNop().Sugar(), name, tracing.AlwaysSample); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}
	c, err := kncloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Fatalf("failed to start receiver: %s", c.StartReceiver(context.Background(), handler))
}
