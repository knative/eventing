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

package aggregator

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/mako/go/quickstore"
	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"knative.dev/pkg/test/mako"

	"knative.dev/eventing/test/common/performance/common"
	pb "knative.dev/eventing/test/common/performance/event_state"
)

const (
	maxRcvMsgSize         = 1024 * 1024 * 1024
	publishFailureMessage = "Publish failure"
	deliverFailureMessage = "Delivery failure"
)

// thread-safe events recording map
type eventsRecord struct {
	sync.RWMutex
	*pb.EventsRecord
}

var fatalf = log.Fatalf

type Aggregator struct {
	// thread-safe events recording maps
	sentEvents     *eventsRecord
	acceptedEvents *eventsRecord
	receivedEvents *eventsRecord

	// channel to notify the main goroutine that an events record has been received
	notifyEventsReceived chan struct{}

	// GRPC server
	listener net.Listener
	server   *grpc.Server

	makoTags      []string
	expectRecords uint
}

func NewAggregator(listenAddr string, expectRecords uint, makoTags []string) (common.Executor, error) {
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}

	executor := &Aggregator{
		listener:             l,
		notifyEventsReceived: make(chan struct{}),
		makoTags:             makoTags,
		expectRecords:        expectRecords,
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
	executor.receivedEvents = &eventsRecord{EventsRecord: &pb.EventsRecord{
		Type:   pb.EventsRecord_RECEIVED,
		Events: make(map[string]*timestamp.Timestamp),
	}}

	return executor, nil
}

func (ag *Aggregator) Run(ctx context.Context) {
	log.Printf("Configuring Mako")

	makoClientCtx, _ := context.WithTimeout(ctx, time.Minute*10)

	client, err := mako.Setup(makoClientCtx, ag.makoTags...)
	if err != nil {
		fatalf("Failed to setup mako: %v", err)
	}

	// Use a fresh context here so that our RPC to terminate the sidecar
	// isn't subject to our timeout (or we won't shut it down when we time out)
	defer client.ShutDownFunc(context.Background())

	// Wrap fatalf in a helper or our sidecar will live forever.
	fatalf = func(f string, args ...interface{}) {
		client.ShutDownFunc(context.Background())
		log.Fatalf(f, args...)
	}

	// --- Run GRPC events receiver
	log.Printf("Starting events recorder server")

	go func() {
		if err := ag.server.Serve(ag.listener); err != nil {
			fatalf("Failed to serve: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		log.Printf("Terminating events recorder server")
		ag.server.GracefulStop()
	}()

	// --- Wait for all records
	log.Printf("Expecting %d events records", ag.expectRecords)
	ag.waitForEvents()
	log.Printf("Received all expected events records")

	ag.server.GracefulStop()

	// --- Publish latencies
	log.Printf("%-15s: %d", "Sent count", len(ag.sentEvents.Events))
	log.Printf("%-15s: %d", "Accepted count", len(ag.acceptedEvents.Events))
	log.Printf("%-15s: %d", "Received count", len(ag.receivedEvents.Events))

	log.Printf("Publishing latencies")

	// count errors
	publishErrorTimestamps := make([]time.Time, 0)
	deliverErrorTimestamps := make([]time.Time, 0)

	for sentID := range ag.sentEvents.Events {
		timestampSentProto := ag.sentEvents.Events[sentID]
		timestampSent, _ := ptypes.Timestamp(timestampSentProto)

		timestampAcceptedProto, accepted := ag.acceptedEvents.Events[sentID]
		timestampAccepted, _ := ptypes.Timestamp(timestampAcceptedProto)

		timestampReceivedProto, received := ag.receivedEvents.Events[sentID]
		timestampReceived, _ := ptypes.Timestamp(timestampReceivedProto)

		if !accepted {
			publishErrorTimestamps = append(publishErrorTimestamps, timestampSent)

			if qerr := client.Quickstore.AddError(mako.XTime(timestampSent), publishFailureMessage); qerr != nil {
				log.Printf("ERROR AddError for publish-failure: %v", qerr)
			}
			continue
		}

		sendLatency := timestampAccepted.Sub(timestampSent)
		// Uncomment to get CSV directly from this container log
		// fmt.Printf("%f,%d,\n", mako.XTime(timestampSent), sendLatency.Nanoseconds())
		// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
		if qerr := client.Quickstore.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"pl": sendLatency.Seconds()}); qerr != nil {
			log.Printf("ERROR AddSamplePoint for publish-latency: %v", qerr)
		}

		if !received {
			deliverErrorTimestamps = append(deliverErrorTimestamps, timestampSent)

			if qerr := client.Quickstore.AddError(mako.XTime(timestampSent), deliverFailureMessage); qerr != nil {
				log.Printf("ERROR AddError for deliver-failure: %v", qerr)
			}
			continue
		}

		e2eLatency := timestampReceived.Sub(timestampSent)
		// Uncomment to get CSV directly from this container log
		// fmt.Printf("%f,,%d\n", mako.XTime(timestampSent), e2eLatency.Nanoseconds())
		// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
		if qerr := client.Quickstore.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"dl": e2eLatency.Seconds()}); qerr != nil {
			log.Printf("ERROR AddSamplePoint for deliver-latency: %v", qerr)
		}
	}

	log.Printf("%-15s: %d", "Publish failure count", len(publishErrorTimestamps))
	log.Printf("%-15s: %d", "Delivery/Lost failure count", len(deliverErrorTimestamps))

	// --- Publish throughput

	log.Printf("Publishing throughputs")

	sentTimestamps := eventsToTimestampsArray(&ag.sentEvents.Events)
	err = publishThpt(client.Quickstore, sentTimestamps, publishErrorTimestamps, "st", "pet")
	if err != nil {
		log.Printf("ERROR AddSamplePoint for send-throughput: %v", err)
	}

	receivedTimestamps := eventsToTimestampsArray(&ag.receivedEvents.Events)
	err = publishThpt(client.Quickstore, receivedTimestamps, deliverErrorTimestamps, "dt", "det")
	if err != nil {
		log.Printf("ERROR AddSamplePoint for deliver-throughput: %v", err)
	}

	// --- Publish error counts as aggregate metrics

	log.Printf("Publishing aggregates")

	client.Quickstore.AddRunAggregate("pe", float64(len(publishErrorTimestamps)))
	client.Quickstore.AddRunAggregate("de", float64(len(deliverErrorTimestamps)))

	log.Printf("Store to mako")

	if out, err := client.Quickstore.Store(); err != nil {
		fatalf("Failed to store data: %v\noutput: %v", err, out)
	}

	log.Printf("Aggregation completed")
}

func eventsToTimestampsArray(events *map[string]*timestamp.Timestamp) []time.Time {
	values := make([]time.Time, 0, len(*events))
	for _, v := range *events {
		t, _ := ptypes.Timestamp(v)
		values = append(values, t)
	}
	return values
}

func publishThpt(q *quickstore.Quickstore,
	successTimestamps, errorTimestamps []time.Time,
	successMetricName, errorMetricName string,
) error {
	successRateMap := make(map[int64]int64)
	errorRateMap := make(map[int64]int64)
	// Aggregate and calculate the success throughput per second.
	for _, t := range successTimestamps {
		successRateMap[t.Unix()]++
		// By adding the key to errorRateMap in this way, we always report the error throughput per second,
		// even if it's zero.
		if _, ok := errorRateMap[t.Unix()]; !ok {
			errorRateMap[t.Unix()] = 0
		}
	}
	// Aggregate and calculate the error throughput per second.
	for _, t := range errorTimestamps {
		errorRateMap[t.Unix()]++
	}

	// Save the success throughput metric to Mako.
	for ts, cnt := range successRateMap {
		if qerr := q.AddSamplePoint(mako.XTime(time.Unix(ts, 0)), map[string]float64{
			successMetricName: float64(cnt),
		}); qerr != nil {
			return qerr
		}
	}
	// Save the error throughput metric to Mako.
	for ts, cnt := range errorRateMap {
		if qerr := q.AddSamplePoint(mako.XTime(time.Unix(ts, 0)), map[string]float64{
			errorMetricName: float64(cnt),
		}); qerr != nil {
			return qerr
		}
	}

	return nil
}

// waitForEvents blocks until the expected number of events records has been received.
func (ag *Aggregator) waitForEvents() {
	for receivedRecords := uint(0); receivedRecords < ag.expectRecords; receivedRecords++ {
		<-ag.notifyEventsReceived
	}
}

// RecordSentEvents implements event_state.EventsRecorder
func (ag *Aggregator) RecordEvents(_ context.Context, in *pb.EventsRecordList) (*pb.RecordReply, error) {
	defer func() {
		ag.notifyEventsReceived <- struct{}{}
	}()

	for _, recIn := range in.Items {
		recType := recIn.GetType()

		var rec *eventsRecord

		switch recType {
		case pb.EventsRecord_SENT:
			rec = ag.sentEvents
		case pb.EventsRecord_ACCEPTED:
			rec = ag.acceptedEvents
		case pb.EventsRecord_RECEIVED:
			rec = ag.receivedEvents
		default:
			log.Printf("Ignoring events record of type %s", recType)
			continue
		}

		log.Printf("-> Recording %d %s events", uint64(len(recIn.Events)), recType)

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
