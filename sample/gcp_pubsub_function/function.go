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

package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/pkg/event"
)

func newPubsubMessage(ctx context.Context, m *pubsub.Message) {
	// Traditionally, complex data is stored in Pub/Sub data as a Base64 encoded string.
	data, _ := base64.StdEncoding.DecodeString(string(m.Data))
	log.Printf("Received data: %q\n", data)
	if len(m.Attributes) != 0 {
		log.Printf("...and attributes: %v\n", m.Attributes)
	}
}

func main() {
	log.Fatal(http.ListenAndServe(":8080", event.Handler(newPubsubMessage)))
}
