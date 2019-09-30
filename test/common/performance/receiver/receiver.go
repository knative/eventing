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
	"github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"knative.dev/eventing/test/common/performance/common"
	pb "knative.dev/eventing/test/common/performance/event_state"
	"log"
	"net"
	"runtime"
	"time"
)

type Receiver struct {
	receivedCh     chan common.EventTimestamp
	endCh          chan bool
	receivedEvents *pb.EventsRecord

	// aggregator GRPC client
	aggregatorClient *pb.AggregatorClient
}

func NewReceiver(paceFlag string, aggregAddr string) (common.Executor, error) {
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

	return &Receiver{
		receivedCh: make(chan common.EventTimestamp, channelSize),
		endCh:      make(chan bool, 1),
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

	r.processEvents()

	closeReceiver()

	log.Println("Receiver closed")

	log.Printf("%-15s: %d\n", "Received count", len(r.receivedEvents.Events))

	err := r.aggregatorClient.Publish(&pb.EventsRecordList{Items: []*pb.EventsRecord{
		r.receivedEvents,
	}})

	if err != nil {
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
		case _, ok := <-r.endCh:
			if !ok {
				return
			}
		}
	}
}

func (r *Receiver) startCloudEventsReceiver(ctx context.Context) error {
	cli, err := newCloudEventsClient()
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents client: %v", err)
	}

	return cli.StartReceiver(ctx, r.processReceiveEvent)
}

// processReceiveEvent processes the event received by the CloudEvents receiver.
func (r *Receiver) processReceiveEvent(event cloudevents.Event) {
	if event.Type() == common.MeasureEventType {
		r.receivedCh <- common.EventTimestamp{EventId: event.ID(), At: ptypes.TimestampNow()}
	} else if event.Type() == common.GCEventType {
		runtime.GC()
	} else if event.Type() == common.EndEventType {
		// Wait a bit so all messages on wire are processed
		time.AfterFunc(time.Second*5, func() {
			close(r.endCh)
		})
	}
}

func newCloudEventsClient() (client.Client, error) {
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	return cloudevents.NewClient(t)
}

// waitForPortAvailable waits until the given TCP port is available.
func waitForPortAvailable(port string) {
	var free bool
	for i := 0; i < 30; i++ {
		conn, err := net.Dial("tcp", ":"+port)
		if err != nil {
			if _, ok := err.(*net.OpError); ok {
				free = true
				break
			}
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if !free {
		log.Fatalf("Timeout waiting for TCP port %s to become available\n")
	}
}
