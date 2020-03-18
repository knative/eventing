/*
Copyright 2020 The Knative Authors

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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/test/lib"
)

type eventRecorder struct {
	es *eventStore
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{es: newEventStore()}
}

// Start the recordevents REST api server
func (er *eventRecorder) StartServer(port int) {
	http.HandleFunc(lib.GetMinMaxPath, er.handleMinMax)
	http.HandleFunc(lib.GetEntryPath, er.handleGetEntry)
	http.HandleFunc(lib.TrimThroughPath, er.handleTrim)
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// HTTP handler for GetMinMax requests
func (er *eventRecorder) handleMinMax(w http.ResponseWriter, r *http.Request) {
	minAvail, maxSeen := er.es.MinMax()
	minMax := lib.MinMaxResponse{
		MinAvail: minAvail,
		MaxSeen:  maxSeen,
	}
	respBytes, err := json.Marshal(minMax)
	if err != nil {
		log.Panicf("Internal error: json marshal shouldn't fail: (%v) (%+v)", err, minMax)
	}

	w.Header().Set("Content-Type", "text/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

// HTTP handler for TrimThrough requests
func (er *eventRecorder) handleTrim(w http.ResponseWriter, r *http.Request) {
	// If we extend this much at all we should vendor a better mux(gorilla, etc)
	path := strings.TrimLeft(r.URL.Path, "/")
	getPrefix := strings.TrimLeft(lib.TrimThroughPath, "/")
	suffix := strings.TrimLeft(strings.TrimPrefix(path, getPrefix), "/")

	seqNum, err := strconv.ParseInt(suffix, 10, 32)
	if err != nil {
		http.Error(w, "Can't parse event sequence number in request", http.StatusBadRequest)
		return
	}

	err = er.es.TrimThrough(int(seqNum))
	if err != nil {
		http.Error(w, "Invalid sequence number in request to trim", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/json")
	w.WriteHeader(http.StatusOK)
}

// HTTP handler for GetEntry requests
func (er *eventRecorder) handleGetEntry(w http.ResponseWriter, r *http.Request) {
	// If we extend this much at all we should vendor a better mux(gorilla, etc)
	path := strings.TrimLeft(r.URL.Path, "/")
	getPrefix := strings.TrimLeft(lib.GetEntryPath, "/")
	suffix := strings.TrimLeft(strings.TrimPrefix(path, getPrefix), "/")

	seqNum, err := strconv.ParseInt(suffix, 10, 32)
	if err != nil {
		http.Error(w, "Can't parse event sequence number in request", http.StatusBadRequest)
		return
	}

	entryBytes, err := er.es.GetEventInfoBytes(int(seqNum))
	if err != nil {
		http.Error(w, "Couldn't find requested event", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/json")
	w.WriteHeader(http.StatusOK)
	w.Write(entryBytes)
}

// handler for cloudevents
func (er *eventRecorder) handler(ctx context.Context, event cloudevents.Event) {
	cloudevents.HTTPTransportContextFrom(ctx)

	tx := cloudevents.HTTPTransportContextFrom(ctx)

	// Store the event
	er.es.StoreEvent(event, map[string][]string(tx.Header))

	// Print interesting headers and full events for debugging
	header := tx.Header
	headerNameList := lib.InterestingHeaders()
	for _, headerName := range headerNameList {
		if headerValue := header.Get(headerName); headerValue != "" {
			log.Printf("Header %s: %s\n", headerName, headerValue)
		}
	}
	if err := event.Validate(); err == nil {
		log.Printf("eventdetails:\n%s", event.String())
	} else {
		log.Printf("error validating the event: %v", err)
	}
}

func main() {
	er := newEventRecorder()
	er.StartServer(lib.RecordEventsPort)

	logger, _ := zap.NewDevelopment()
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracing.AlwaysSample); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}
	c, err := kncloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("Failed to create client, %v", err)
	}

	log.Fatalf("Failed to start receiver: %s", c.StartReceiver(context.Background(), er.handler))
}
