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
	"fmt"
	"log"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
)

func handler(event cloudevents.Event) {
	// TODO: in version 0.5.0 of cloudevents, below can be deleted.

	ctx := event.Context.AsV02()
	var data []byte
	var ok bool
	if data, ok = event.Data.([]byte); !ok {
		fmt.Printf("Got Data Error")
		return
	}
	log.Printf("[%s] %s %s: %+v", ctx.Time.String(), *ctx.ContentType, ctx.Source.String(), string(data))
}

func main() {
	c, err := client.NewDefault()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Printf("will listen on :8080\n")
	log.Fatalf("failed to start receiver: %s", c.StartReceiver(context.Background(), handler))
}
