/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sender

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"

	"knative.dev/eventing/test/performance/infra/common"
	pb "knative.dev/eventing/test/performance/infra/event_state"
)

const (
	defaultEventSource = "perf-test-event-source"
	warmupRps          = 100
	podNameEnvVar      = "POD_NAME"
)

type Sender struct {
	paceSpecs     []common.PaceSpec
	msgSize       uint
	warmupSeconds uint
	fixedBody     bool

	// EventTimestamp channels
	sentCh     chan common.EventTimestamp
	acceptedCh chan common.EventTimestamp

	// events recording maps
	sentEvents     *pb.EventsRecord
	acceptedEvents *pb.EventsRecord

	// load generator
	loadGenerator LoadGenerator

	// aggregator GRPC client
	aggregatorClient *pb.AggregatorClient
}

func NewSender(loadGeneratorFactory LoadGeneratorFactory, aggregAddr string, msgSize uint, warmupSeconds uint, paceFlag string, fixedBody bool) (common.Executor, error) {
	pacerSpecs, err := common.ParsePaceSpec(paceFlag)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pace spec: %v", err)
	}

	// create a connection to the aggregator
	aggregatorClient, err := pb.NewAggregatorClient(aggregAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the aggregator: %v", err)
	}

	// We need those estimates to allocate memory before benchmark starts
	estimatedNumberOfMessagesInsideAChannel, estimatedNumberOfTotalMessages := common.CalculateMemoryConstraintsForPaceSpecs(pacerSpecs)

	// Small note: receivedCh depends on receive thpt and not send thpt but we
	// don't care since this is a pessimistic estimate and receive thpt < send thpt
	// PS after 3 weeks: Yeah I know this is not an entirely true assumption after the system becomes
	// unstable, but we are interested to understand when the system becomes unstable,
	// not what happens after

	executor := &Sender{
		msgSize:       msgSize,
		warmupSeconds: warmupSeconds,
		paceSpecs:     pacerSpecs,
		fixedBody:     fixedBody,

		sentCh:     make(chan common.EventTimestamp, estimatedNumberOfMessagesInsideAChannel),
		acceptedCh: make(chan common.EventTimestamp, estimatedNumberOfMessagesInsideAChannel),

		sentEvents: &pb.EventsRecord{
			Type:   pb.EventsRecord_SENT,
			Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
		},
		acceptedEvents: &pb.EventsRecord{
			Type:   pb.EventsRecord_ACCEPTED,
			Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
		},

		aggregatorClient: aggregatorClient,
	}

	executor.loadGenerator, err = loadGeneratorFactory(eventsSource(), executor.sentCh, executor.acceptedCh)
	if err != nil {
		return nil, err
	}

	return executor, nil
}

func (s *Sender) Run(ctx context.Context) {
	// --- Warmup phase
	log.Printf("--- BEGIN WARMUP ---")
	if s.warmupSeconds > 0 {
		if err := s.warmup(ctx, s.warmupSeconds); err != nil {
			log.Fatalf("Failed to run warmup: %v", err)
		}
	} else {
		log.Printf("Warmup skipped")
	}
	log.Printf("---- END WARMUP ----")

	log.Printf("--- BEGIN BENCHMARK ---")

	// Start the events processor
	log.Println("Starting events processor")
	go s.processEvents()

	// Clean mess before starting
	runtime.GC()

	log.Println("Starting benchmark")

	// Run all pace configurations
	benchmarkBeginning := time.Now()
	for i, pace := range s.paceSpecs {
		log.Printf("Starting pace %d° at %v rps for %v seconds", i+1, pace.Rps, pace.Duration)
		s.loadGenerator.RunPace(i, pace, s.msgSize, s.fixedBody)

		// Wait for flush
		time.Sleep(common.WaitForFlush)

		// Trigger GC
		log.Println("Triggering GC")
		s.loadGenerator.SendGCEvent()
		runtime.GC()

		// Wait for receivers GC
		time.Sleep(common.WaitForReceiverGC)
	}

	s.loadGenerator.SendEndEvent()

	log.Printf("Benchmark completed in %v", time.Since(benchmarkBeginning))

	s.closeChannels()

	log.Println("---- END BENCHMARK ----")

	log.Println("Sending collected data to the aggregator")

	log.Printf("%-15s: %d", "Sent count", len(s.sentEvents.Events))
	log.Printf("%-15s: %d", "Accepted count", len(s.acceptedEvents.Events))

	err := s.aggregatorClient.Publish(&pb.EventsRecordList{Items: []*pb.EventsRecord{
		s.sentEvents,
		s.acceptedEvents,
	}})
	if err != nil {
		log.Fatalf("Failed to send events record: %v\n", err)
	}
}

func (s *Sender) warmup(ctx context.Context, warmupSeconds uint) error {
	log.Println("Starting warmup")

	s.loadGenerator.Warmup(common.PaceSpec{Rps: warmupRps, Duration: time.Duration(warmupSeconds) * time.Second}, s.msgSize, s.fixedBody)

	// give the channel some time to drain the events it may still have enqueued
	time.Sleep(common.WaitAfterWarmup)

	return nil
}

func (s *Sender) closeChannels() {
	log.Printf("All requests sent")

	close(s.sentCh)
	close(s.acceptedCh)

	log.Printf("All channels closed")
}

// processEvents keeps a record of all events (sent, accepted, failed, received).
func (s *Sender) processEvents() {
	for {
		select {
		case e, ok := <-s.sentCh:
			if !ok {
				continue
			}
			s.sentEvents.Events[e.EventId] = e.At

		case e, ok := <-s.acceptedCh:
			if !ok {
				continue
			}
			s.acceptedEvents.Events[e.EventId] = e.At
		}
	}
}

func eventsSource() string {
	if pn := os.Getenv(podNameEnvVar); pn != "" {
		return pn
	}
	return defaultEventSource
}
