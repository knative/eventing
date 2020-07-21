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
	"os"
	"strconv"
	"strings"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/kncloudevents"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/dropevents"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/test_images"
)

type eventRecorder struct {
	es *eventStore
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{es: newEventStore()}
}

// Start the recordevents REST api server
func (er *eventRecorder) StartServer(port int) {
	http.HandleFunc(recordevents.GetMinMaxPath, er.handleMinMax)
	http.HandleFunc(recordevents.GetEntryPath, er.handleGetEntry)
	http.HandleFunc(recordevents.TrimThroughPath, er.handleTrim)
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// HTTP handler for GetMinMax requests
func (er *eventRecorder) handleMinMax(w http.ResponseWriter, r *http.Request) {
	minAvail, maxSeen := er.es.MinMax()
	minMax := recordevents.MinMaxResponse{
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
	getPrefix := strings.TrimLeft(recordevents.TrimThroughPath, "/")
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
	getPrefix := strings.TrimLeft(recordevents.GetEntryPath, "/")
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

func (er *eventRecorder) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

	event, eventErr := cloudeventsbindings.ToEvent(context.TODO(), m)
	header := request.Header

	er.es.StoreEvent(event, eventErr, map[string][]string(header))

	headerNameList := testlib.InterestingHeaders()
	for _, headerName := range headerNameList {
		if headerValue := header.Get(headerName); headerValue != "" {
			log.Printf("Header %s: %s\n", headerName, headerValue)
		}
	}

	if eventErr != nil {
		log.Printf("error receiving the event: %v", eventErr)
	} else {
		valErr := event.Validate()
		if valErr == nil {
			log.Printf("eventdetails:\n%s", event.String())
		} else {
			log.Printf("error validating the event: %v", valErr)
		}
	}

	writer.WriteHeader(http.StatusAccepted)
}

func main() {
	er := newEventRecorder()
	er.StartServer(recordevents.RecordEventsPort)

	logger, _ := zap.NewDevelopment()
	if err := test_images.ConfigureTracing(logger.Sugar(), ""); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	algorithm, ok := os.LookupEnv(dropevents.SkipAlgorithmKey)
	handler := kncloudevents.CreateHandler(er)
	if ok {
		skipper := dropevents.SkipperAlgorithm(algorithm)
		counter := dropevents.CounterHandler{
			Skipper: skipper,
		}
		next := handler
		handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if counter.Skip() {
				writer.WriteHeader(http.StatusConflict)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}

	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		panic(err)
	}
}
