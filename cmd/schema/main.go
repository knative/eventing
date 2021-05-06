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
	"log"

	"knative.dev/hack/schema/commands"
	"knative.dev/hack/schema/registry"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
)

// schema is a tool to dump the schema for Eventing resources.
func main() {
	// Eventing
	registry.Register(&eventingv1.Broker{})
	registry.Register(&eventingv1.Trigger{})

	// Messaging
	registry.Register(&messagingv1.Subscription{})
	registry.Register(&messagingv1.Channel{})
	registry.Register(&messagingv1.InMemoryChannel{})

	// Sources
	registry.Register(&sourcesv1.SinkBinding{})

	if err := commands.New("knative.dev/eventing").Execute(); err != nil {
		log.Fatal("Error during command execution: ", err)
	}
}
