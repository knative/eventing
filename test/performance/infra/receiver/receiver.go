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

package receiver

import (
	"context"
	"fmt"
	"log"
	"net"
	"runtime"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"knative.dev/eventing/test/performance/infra/common"
	pb "knative.dev/eventing/test/performance/infra/event_state"
)

const shutdownWaitTime = time.Second * 5

// Receiver records the received events and sends to the aggregator.
// Since sender implementations can put id and type of event inside the event payload,
// then the Receiver uses IdExtractor and TypeExtractor to extract them
type Receiver struct {
	typeExtractor TypeExtractor
	idExtractor   IdExtractor
	timeout       time.Duration

	receivedCh     chan common.EventTimestamp
	endCh          chan struct{}
	receivedEvents *pb.EventsRecord

	// aggregator GRPC client
	aggregatorClient *pb.AggregatorClient
}

func NewReceiver(paceFlag string, aggregAddr string, warmupSeconds uint, typeExtractor TypeExtractor, idExtractor IdExtractor) (common.Executor, error) {
	pace, err := common.ParsePaceSpec(paceFlag)
	if err != nil {
		return nil, err
	}

	// create a connection to the aggregator
	aggregatorClient, err := pb.NewAggregatorClient(aggregAddr)
	if err != nil {
		return nil, err
	}

	channelSize, totalMessages := common.CalculateMemoryConstraintsForPaceSpecs(pace)

	// Calculate timeout for receiver
	var timeout time.Duration
	timeout = time.Second * time.Duration(warmupSeconds)
	if timeout != 0 {
		timeout += common.WaitAfterWarmup
	}
	for _, p := range pace {
		timeout += p.Duration + common.WaitForFlush + common.WaitForReceiverGC
	}
	// The timeout is doubled because the sender is slowed down by the SUT when the load is too high and test requires more than needed.
	// Coefficient of 2 is based on experimental evidence.
	// More: https://github.com/knative/eventing/pull/2195#discussion_r348368914
	timeout *= 2

	return &Receiver{
		typeExtractor: typeExtractor,
		idExtractor:   idExtractor,
		timeout:       timeout,
		receivedCh:    make(chan common.EventTimestamp, channelSize),
		endCh:         make(chan struct{}, 1),
		receivedEvents: &pb.EventsRecord{
			Type:   pb.EventsRecord_RECEIVED,
			Events: make(map[string]*timestamp.Timestamp, totalMessages),
		},
		aggregatorClient: aggregatorClient,
	}, nil
}

func (r *Receiver) Run(ctx context.Context) {
	// Wait the port before starting the ce receiver
	waitForPortAvailable(common.CEReceiverPort)

	receiverCtx, closeReceiver := context.WithCancel(ctx)

	go func() {
		if err := r.startCloudEventsReceiver(receiverCtx); err != nil {
			log.Fatalf("Failed to start CloudEvents receiver: %v", err)
		}
	}()

	// When the testing service is degraded, there is a chance that the end message is not received
	// This timer sends to endCh a signal to stop processing events and start tear down of receiver
	timeoutTimer := time.AfterFunc(r.timeout, func() {
		log.Printf("Receiver timeout")
		r.endCh <- struct{}{}
	})
	log.Printf("Started receiver timeout timer of duration %v", r.timeout)

	r.processEvents()

	// Stop the timeoutTimer in case the tear down was triggered by end message
	timeoutTimer.Stop()

	closeReceiver()

	log.Println("Receiver closed")

	log.Printf("%-15s: %d", "Received count", len(r.receivedEvents.Events))

	if err := r.aggregatorClient.Publish(&pb.EventsRecordList{Items: []*pb.EventsRecord{
		r.receivedEvents,
	}}); err != nil {
		log.Fatalf("Failed to send events record: %v\n", err)
	}

	close(r.receivedCh)
}

func (r *Receiver) processEvents() {
	for {
		select {
		case e, ok := <-r.receivedCh:
			if !ok {
				return
			}
			r.receivedEvents.Events[e.EventId] = e.At
		case <-r.endCh:
			return
		}
	}
}

func (r *Receiver) startCloudEventsReceiver(ctx context.Context) error {
	cli, err := cloudevents.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents client: %v", err)
	}

	log.Printf("CloudEvents receiver started")
	return cli.StartReceiver(ctx, r.processReceiveEvent)
}

// processReceiveEvent processes the event received by the CloudEvents receiver.
func (r *Receiver) processReceiveEvent(event cloudevents.Event) {
	t := r.typeExtractor(event)
	switch t {
	case common.MeasureEventType:
		r.receivedCh <- common.EventTimestamp{EventId: r.idExtractor(event), At: ptypes.TimestampNow()}
	case common.GCEventType:
		runtime.GC()
	case common.EndEventType:
		log.Printf("End message received correctly")
		// Wait a bit so all messages on wire are processed
		time.AfterFunc(shutdownWaitTime, func() {
			r.endCh <- struct{}{}
		})
	}
}

// waitForPortAvailable waits until the given TCP port is available.
func waitForPortAvailable(port string) {
	var free bool
	for i := 0; i < 30; i++ {
		conn, err := net.Dial("tcp", ":"+port)
		if _, ok := err.(*net.OpError); ok {
			free = true
			break
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if !free {
		log.Fatalf("Timeout waiting for TCP port %s to become available\n", port)
	}
}
