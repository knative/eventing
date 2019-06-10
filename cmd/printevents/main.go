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

// A simple event target that prints events.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	ce "github.com/cloudevents/sdk-go"
	"github.com/knative/eventing/pkg/kncloudevents/kntransport"
)

func main() {
	port := flag.Int("port", 8080, "listening port")
	flag.Parse()
	if len(flag.Args()) != 0 {
		flag.Usage()
		os.Exit(1)
	}
	t, err := ce.NewHTTPTransport(ce.WithPort(*port))
	if err != nil {
		log.Fatalf("failed to create transport: %v", err)
	}
	t.SetReceiver(kntransport.ReceiveFunc(
		func(ctx context.Context, e ce.Event, er *ce.EventResponse) error {
			log.Printf("%v\n", e)
			return nil
		}))
	log.Printf("listening on port %d", *port)
	if err := t.StartReceiver(context.Background()); err != nil {
		log.Fatalf("receiver error: %v", err)
	}
	log.Printf("receiver returned")
}
