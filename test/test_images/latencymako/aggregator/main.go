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

//go:generate protoc -I ../ --go_out=plugins=grpc:../ ../event_state.proto

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes/timestamp"

	pb "knative.dev/eventing/test/test_images/latencymako"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/test/mako"
)

const listenAddr = ":10000"

// flags for the image
var (
	verbose       bool
	expectRecords uint
)

var fatalf = log.Fatalf

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.UintVar(&expectRecords, "expect-records", 4, "Number of expected events records before aggregating data.")
}

// aggregation of received events records
type eventsRecord struct {
	sync.RWMutex
	pb.EventsRecord
}

var (
	sentEvents     eventsRecord
	acceptedEvents eventsRecord
	failedEvents   eventsRecord
	receivedEvents eventsRecord
)

var notifyEventsReceived = make(chan struct{})

func main() {
	flag.Parse()

	// --- Configure mako

	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	printf("Configuring Mako")

	// Use the benchmark key created
	ctx, _, qclose, err := mako.Setup(ctx)
	if err != nil {
		fatalf("Failed to setup mako: %v", err)
	}

	// Use a fresh context here so that our RPC to terminate the sidecar
	// isn't subject to our timeout (or we won't shut it down when we time out)
	defer qclose(context.Background())

	// Wrap fatalf in a helper or our sidecar will live forever.
	fatalf = func(f string, args ...interface{}) {
		qclose(context.Background())
		fatalf(f, args...)
	}

	// --- Initialize records maps

	sentEvents.Events = make(map[string]*timestamp.Timestamp)
	acceptedEvents.Events = make(map[string]*timestamp.Timestamp)
	failedEvents.Events = make(map[string]*timestamp.Timestamp)
	receivedEvents.Events = make(map[string]*timestamp.Timestamp)

	// --- Run GRPC events receiver

	printf("Starting events recorder server")

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pb.RegisterEventsRecorderServer(s, &server{})

	go func() {
		if err := s.Serve(l); err != nil {
			fatalf("Failed to serve: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		printf("Terminating events recorder server")
		s.GracefulStop()
	}()

	printf("Expecting %d events records", expectRecords)
	waitForEvents()
	printf("Received all expected events records")

	s.GracefulStop()

	printf("%-15s: %d", "Sent count", len(sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(failedEvents.Events))
	printf("%-15s: %d", "Received count", len(receivedEvents.Events))
}

// waitForEvents blocks until the expected number of events records have been received.
func waitForEvents() {
	for receivedRecords := uint(0); receivedRecords < expectRecords; receivedRecords++ {
		<-notifyEventsReceived
	}
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
}

// server is used to implement latencymako.EventsRecorder
type server struct{}

// RecordSentEvents implements latencymako.EventsRecorder
func (s *server) RecordEvents(_ context.Context, in *pb.EventsRecord) (*pb.RecordReply, error) {
	defer func() { notifyEventsReceived <- struct{}{} }()

	count := uint64(len(in.GetEvents()))
	recType := in.GetType()

	var rec *eventsRecord

	switch recType {
	case pb.EventsRecord_SENT:
		rec = &sentEvents
	case pb.EventsRecord_ACCEPTED:
		rec = &acceptedEvents
	case pb.EventsRecord_FAILED:
		rec = &failedEvents
	case pb.EventsRecord_RECEIVED:
		rec = &receivedEvents
	}

	printf("-> Recording %d %s events", count, recType)

	rec.Lock()
	defer rec.Unlock()
	for id, t := range in.Events {
		if _, exists := rec.Events[id]; exists {
			log.Printf("!! Found duplicate %s event ID %s", recType, id)
			continue
		}
		rec.Events[id] = t
	}

	return &pb.RecordReply{Count: count}, nil
}
