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
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	pb "knative.dev/eventing/test/test_images/latencymako/event_state"
	"knative.dev/pkg/test/mako"
)

const maxRcvMsgSize = 1024 * 1024 * 10

// thread-safe events recording map
type eventsRecord struct {
	sync.RWMutex
	*pb.EventsRecord
}

type aggregatorExecutor struct {
	// thread-safe events recording maps
	sentEvents     *eventsRecord
	acceptedEvents *eventsRecord
	failedEvents   *eventsRecord
	receivedEvents *eventsRecord

	// channel to notify the main goroutine that an events record has been received
	notifyEventsReceived chan struct{}

	// GRPC server
	listener net.Listener
	server   *grpc.Server
}

func newAggregatorExecutor(lis net.Listener) testExecutor {
	executor := &aggregatorExecutor{
		listener:             lis,
		notifyEventsReceived: make(chan struct{}),
	}

	// --- Create GRPC server

	s := grpc.NewServer(grpc.MaxRecvMsgSize(maxRcvMsgSize))
	pb.RegisterEventsRecorderServer(s, executor)
	executor.server = s

	// --- Initialize records maps

	executor.sentEvents = &eventsRecord{EventsRecord: &pb.EventsRecord{
		Type:   pb.EventsRecord_SENT,
		Events: make(map[string]*timestamp.Timestamp),
	}}
	executor.acceptedEvents = &eventsRecord{EventsRecord: &pb.EventsRecord{
		Type:   pb.EventsRecord_ACCEPTED,
		Events: make(map[string]*timestamp.Timestamp),
	}}
	executor.failedEvents = &eventsRecord{EventsRecord: &pb.EventsRecord{
		Type:   pb.EventsRecord_FAILED,
		Events: make(map[string]*timestamp.Timestamp),
	}}
	executor.receivedEvents = &eventsRecord{EventsRecord: &pb.EventsRecord{
		Type:   pb.EventsRecord_RECEIVED,
		Events: make(map[string]*timestamp.Timestamp),
	}}

	return executor
}

func (ex *aggregatorExecutor) Run(ctx context.Context) {
	// --- Configure mako

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

	// --- Run GRPC events receiver

	printf("Starting events recorder server")

	go func() {
		if err := ex.server.Serve(ex.listener); err != nil {
			fatalf("Failed to serve: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		printf("Terminating events recorder server")
		ex.server.GracefulStop()
	}()

	printf("Expecting %d events records", expectRecords)
	ex.waitForEvents()
	printf("Received all expected events records")

	ex.server.GracefulStop()

	// --- Publish results

	printf("%-15s: %d", "Sent count", len(ex.sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(ex.acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(ex.failedEvents.Events))
	printf("%-15s: %d", "Received count", len(ex.receivedEvents.Events))

	printf("Publishing data points to Mako")

	// count errors
	var publishErrorCount int
	var deliverErrorCount int

	for sentID := range ex.sentEvents.Events {
		timestampSentProto := ex.sentEvents.Events[sentID]
		timestampSent, _ := ptypes.Timestamp(timestampSentProto)

		timestampAcceptedProto, accepted := ex.acceptedEvents.Events[sentID]
		timestampAccepted, _ := ptypes.Timestamp(timestampAcceptedProto)

		timestampReceivedProto, received := ex.receivedEvents.Events[sentID]
		timestampReceived, _ := ptypes.Timestamp(timestampReceivedProto)

		if !accepted {
			errMsg := "Failed on broker"
			if _, failed := ex.failedEvents.Events[sentID]; !failed {
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

// waitForEvents blocks until the expected number of events records has been received.
func (ex *aggregatorExecutor) waitForEvents() {
	for receivedRecords := uint(0); receivedRecords < expectRecords; receivedRecords++ {
		<-ex.notifyEventsReceived
	}
}

// RecordSentEvents implements event_state.EventsRecorder
func (ex *aggregatorExecutor) RecordEvents(_ context.Context, in *pb.EventsRecordList) (*pb.RecordReply, error) {
	defer func() {
		ex.notifyEventsReceived <- struct{}{}
	}()

	for _, recIn := range in.Items {
		recType := recIn.GetType()

		var rec *eventsRecord

		switch recType {
		case pb.EventsRecord_SENT:
			rec = ex.sentEvents
		case pb.EventsRecord_ACCEPTED:
			rec = ex.acceptedEvents
		case pb.EventsRecord_FAILED:
			rec = ex.failedEvents
		case pb.EventsRecord_RECEIVED:
			rec = ex.receivedEvents
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
