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
	"log"
	"net/http"

	"github.com/knative/eventing/pkg/event"
)

func newSlackEvent(ctx context.Context, m *interface{}) {
	log.Printf("Received data: %q\n", m)
}

func main() {
	log.Print("Adam's bad")

	log.Fatal(http.ListenAndServe(":8080", event.Handler(newSlackEvent)))
}
