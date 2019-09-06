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

//go:generate protoc -I ./event_state --go_out=plugins=grpc:./event_state ./event_state/event_state.proto

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	pb "knative.dev/eventing/test/test_images/latencymako_aggregator/event_state"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/test/mako"
)

const listenAddr = ":10000"
const maxRcvMsgSize = 1024 * 1024 * 10

// flags for the image
var (
	verbose       bool
	expectRecords uint
)

var fatalf = log.Fatalf

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.UintVar(&expectRecords, "expect-records", 1, "Number of expected events records before aggregating data.")
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
	ctx, q, qclose, err := mako.Setup(ctx)
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

	s := grpc.NewServer(grpc.MaxRecvMsgSize(maxRcvMsgSize))

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

	// --- Publish results

	printf("%-15s: %d", "Sent count", len(sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(failedEvents.Events))
	printf("%-15s: %d", "Received count", len(receivedEvents.Events))

	printf("Publishing data points to Mako")

	// count errors
	var publishErrorCount int
	var deliverErrorCount int

	for sentID := range sentEvents.Events {
		timestampSentProto := sentEvents.Events[sentID]
		timestampSent, _ := ptypes.Timestamp(timestampSentProto)

		timestampAcceptedProto, accepted := acceptedEvents.Events[sentID]
		timestampAccepted, _ := ptypes.Timestamp(timestampAcceptedProto)

		timestampReceivedProto, received := receivedEvents.Events[sentID]
		timestampReceived, _ := ptypes.Timestamp(timestampReceivedProto)

		if !accepted {
			errMsg := "Failed on broker"
			if _, failed := failedEvents.Events[sentID]; !failed {
				// TODO(antoineco): should never happen, check whether the failed map makes any sense
				errMsg = "Event not accepted but missing from failed map"
			}

			deliverErrorCount++

			if qerr := q.AddError(mako.XTime(timestampSent), errMsg); qerr != nil {
				log.Printf("ERROR AddError: %v", qerr)
			}
			continue
		}

		sendLatency := timestampAccepted.Sub(timestampSent)
		// Uncomment to get CSV directly from this container log
		//fmt.Printf("%f,%d,\n", mako.XTime(timestampSent), sendLatency.Nanoseconds())
		// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
		if qerr := q.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"pl": sendLatency.Seconds()}); qerr != nil {
			log.Printf("ERROR AddSamplePoint: %v", qerr)
		}

		if !received {
			publishErrorCount++

			if qerr := q.AddError(mako.XTime(timestampSent), "Event not delivered"); qerr != nil {
				log.Printf("ERROR AddError: %v", qerr)
			}
			continue
		}

		e2eLatency := timestampReceived.Sub(timestampSent)
		// Uncomment to get CSV directly from this container log
		//fmt.Printf("%f,,%d\n", mako.XTime(timestampSent), e2eLatency.Nanoseconds())
		// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
		if qerr := q.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"dl": e2eLatency.Seconds()}); qerr != nil {
			log.Printf("ERROR AddSamplePoint: %v", qerr)
		}
	}

	// publish error counts as aggregate metrics
	q.AddRunAggregate("pe", float64(publishErrorCount))
	q.AddRunAggregate("de", float64(deliverErrorCount))

	if out, err := q.Store(); err != nil {
		fatalf("Failed to store data: %v\noutput: %v", err, out)
	}
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
func (s *server) RecordEvents(_ context.Context, in *pb.EventsRecordList) (*pb.RecordReply, error) {
	defer func() { notifyEventsReceived <- struct{}{} }()

	for _, recIn := range in.Items {
		recType := recIn.GetType()

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
		default:
			printf("Ignoring events record of type %s", recType)
			continue
		}

		printf("-> Recording %d %s events", uint64(len(recIn.Events)), recType)

		func() {
			rec.Lock()
			defer rec.Unlock()
			for id, t := range recIn.Events {
				if _, exists := rec.Events[id]; exists {
					log.Printf("!! Found duplicate %s event ID %s", recType, id)
					continue
				}
				rec.Events[id] = t
			}
		}()
	}

	return &pb.RecordReply{Count: uint32(len(in.Items))}, nil
}
