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
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go"
)

func getHeader(header http.Header, headerName string) string {
	&value := string(nil)
	if header != nil && len(header) > 0 {
		for k, v := range header {
			if k == headerName {
				*value = v				
			}
		}
	}
	return value
}

//func handler(event cloudevents.Event) {
func handler(ctx context.Context, event cloudevents.Event) {
	fmt.Printf("Got Event Context: %+v\n", event.Context)
	tx := cloudevents.HTTPTransportContextFrom(ctx)
	fmt.Printf("Got Transport Context: %+v\n", tx)
	fmt.Printf("----------------------------\n")
	header := tx.Header()

	if err := event.Validate(); err == nil {
		fmt.Printf("eventdetails:\n%s", event.String())
	} else {
		log.Printf("error validating the event: %v", err)
	}
}

func main() {
	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create eventdetails client, %v", err)
	}

	log.Fatalf("failed to start eventdetails receiver: %s", c.StartReceiver(context.Background(), handler))
}
