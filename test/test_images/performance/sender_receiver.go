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
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	ceclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	vegeta "github.com/tsenart/vegeta/lib"

	"knative.dev/eventing/test/common"
	pb "knative.dev/eventing/test/test_images/performance/event_state"
	pkgtest "knative.dev/pkg/test"
	pkgpacers "knative.dev/pkg/test/vegeta/pacers"
)

const (
	warmupEventType       = "warmup.perf-test"
	benchmarkEventType    = "continue.perf-test"
	defaultEventSource    = "perf-test-event-source"
	defaultCEReceiverPort = "8080"
	warmupRps             = 100
	defaultDuration       = 10 * time.Second
	defaultTestNamespace  = "default"
)

// environment variables consumed by the sender-receiver
const (
	podNameEnvVar      = "POD_NAME"
	podNamespaceEnvVar = "POD_NAMESPACE"
)

// state is the recorded state of an event.
type state struct {
	eventId string
	at      *timestamp.Timestamp
}

type (
	sentState     state
	acceptedState state
	failedState   state
	receivedState state
)

// Vegeta pace specification
type paceSpec struct {
	rps      int
	duration time.Duration
}

type attackSpec struct {
	pacer    vegeta.Pacer
	duration time.Duration
}

func (a attackSpec) Attack(i int, targeter vegeta.Targeter, attacker *vegeta.Attacker) {
	printf("Starting attack %d° with pace %v rps for %v seconds", i+1, a.pacer, a.duration)
	res := attacker.Attack(targeter, a.pacer, a.duration, fmt.Sprintf("%s-attack-%d", eventsSource(), i))
	for range res {
	}

	// Wait for flush
	time.Sleep(1 * time.Second)

	// Trigger GC
	printf("Triggering GC")
	runtime.GC()
}

type senderReceiverExecutor struct {
	// state channels
	sentCh     chan sentState
	acceptedCh chan acceptedState
	failedCh   chan failedState
	receivedCh chan receivedState

	// events recording maps
	sentEvents     *pb.EventsRecord
	acceptedEvents *pb.EventsRecord
	failedEvents   *pb.EventsRecord
	receivedEvents *pb.EventsRecord

	// Vegeta attacker
	attacker   *vegeta.Attacker
	pacerSpecs []paceSpec

	// aggregator GRPC client
	aggCli pb.EventsRecorderClient
}

func newSenderReceiverExecutor(ps []paceSpec, aggCli pb.EventsRecorderClient) testExecutor {
	executor := &senderReceiverExecutor{
		pacerSpecs: ps,
		aggCli:     aggCli,
	}

	// --- Allocate channels

	printf("Configuring channels")

	// We need those estimates to allocate memory before benchmark starts
	var estimatedNumberOfMessagesInsideAChannel uint64
	var estimatedNumberOfTotalMessages uint64

	for _, pacer := range ps {
		totalMessages := uint64(pacer.rps * int(pacer.duration.Seconds()))
		// Add a bit more, just to be sure that we don't under allocate
		totalMessages = totalMessages + uint64(float64(totalMessages)*0.1)
		// Queueing theory: given our channels can process 50 rps, queueLength = arrival rps / 50 rps = pacer.rps / 50
		queueLength := uint64(pacer.rps / 50)
		if queueLength < 10 {
			queueLength = 10
		}
		estimatedNumberOfTotalMessages += totalMessages
		if queueLength > estimatedNumberOfMessagesInsideAChannel {
			estimatedNumberOfMessagesInsideAChannel = queueLength
		}
	}

	printf("Estimated channel size: %v", estimatedNumberOfMessagesInsideAChannel)
	printf("Estimated total messages: %v", estimatedNumberOfTotalMessages)

	// Create all channels
	executor.sentCh = make(chan sentState, estimatedNumberOfMessagesInsideAChannel)
	executor.acceptedCh = make(chan acceptedState, estimatedNumberOfMessagesInsideAChannel)
	executor.failedCh = make(chan failedState, estimatedNumberOfMessagesInsideAChannel)
	executor.receivedCh = make(chan receivedState, estimatedNumberOfMessagesInsideAChannel)

	// Small note: receivedCh depends on receive thpt and not send thpt but we
	// don't care since this is a pessimistic estimate and receive thpt < send thpt

	// Initialize records maps
	executor.sentEvents = &pb.EventsRecord{
		Type:   pb.EventsRecord_SENT,
		Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
	}
	executor.acceptedEvents = &pb.EventsRecord{
		Type:   pb.EventsRecord_ACCEPTED,
		Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
	}
	executor.failedEvents = &pb.EventsRecord{
		Type:   pb.EventsRecord_FAILED,
		Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
	}
	executor.receivedEvents = &pb.EventsRecord{
		Type:   pb.EventsRecord_RECEIVED,
		Events: make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages),
	}

	// --- Create Vegeta attacker

	client := http.Client{Transport: requestInterceptor{
		before: func(request *http.Request) {
			id := request.Header.Get("Ce-Id")
			executor.sentCh <- sentState{eventId: id, at: ptypes.TimestampNow()}
		},
		after: func(request *http.Request, response *http.Response, e error) {
			id := request.Header.Get("Ce-Id")
			t := ptypes.TimestampNow()
			if e != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				executor.failedCh <- failedState{eventId: id, at: t}
			} else {
				executor.acceptedCh <- acceptedState{eventId: id, at: t}
			}
		},
	}}

	executor.attacker = vegeta.NewAttacker(
		vegeta.Client(&client),
		vegeta.Workers(workers),
	)

	return executor
}

func (ex *senderReceiverExecutor) Run(ctx context.Context) {
	// --- Warmup phase

	printf("--- BEGIN WARMUP ---")
	if warmupSeconds > 0 {
		if err := ex.warmup(ctx, warmupSeconds); err != nil {
			fatalf("Failed to run warmup: %v", err)
		}
	} else {
		printf("Warmup skipped")
	}
	printf("---- END WARMUP ----")

	printf("--- BEGIN BENCHMARK ---")

	// Start the events receiver
	printf("Starting CloudEvents receiver")

	waitForPortAvailable(defaultCEReceiverPort)

	go func() {
		if err := ex.startCloudEventsReceiver(ctx, ex.processReceiveEvent); err != nil {
			fatalf("Failed to start CloudEvents receiver: %v", err)
		}
	}()

	// Start the events processor
	printf("Starting events processor")
	go ex.processEvents()

	// Clean mess before starting
	runtime.GC()

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, benchmarkEventType, eventsSource()).VegetaTargeter()

	attacks := make([]attackSpec, len(ex.pacerSpecs))
	var totalBenchmarkDuration time.Duration

	for i, ps := range ex.pacerSpecs {
		attacks[i] = attackSpec{pacer: vegeta.ConstantPacer{Freq: ps.rps, Per: time.Second}, duration: ps.duration}
		printf("%d° pace: %d rps for %v seconds", i+1, ps.rps, ps.duration)
		totalBenchmarkDuration += ps.duration
	}

	printf("Total benchmark duration: %v", totalBenchmarkDuration.Seconds())

	printf("Starting benchmark")

	// Run all attacks
	for i, f := range attacks {
		f.Attack(i, targeter, ex.attacker)
	}

	ex.closeChannels()

	printf("---- END BENCHMARK ----")

	printf("Sending collected data to the aggregator")

	printf("%-15s: %d", "Sent count", len(ex.sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(ex.acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(ex.failedEvents.Events))
	printf("%-15s: %d", "Received count", len(ex.receivedEvents.Events))

	err := ex.sendEventsRecordList(&pb.EventsRecordList{Items: []*pb.EventsRecord{
		ex.sentEvents,
		ex.acceptedEvents,
		ex.failedEvents,
		ex.receivedEvents,
	}})
	if err != nil {
		fatalf("Failed to send events record: %v", err)
	}
}

type receiverFn func(cloudevents.Event)

func (ex *senderReceiverExecutor) startCloudEventsReceiver(ctx context.Context, eventHandler receiverFn) error {
	cli, err := newCloudEventsClient(sinkURL)
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents client: %v", err)
	}

	return cli.StartReceiver(ctx, eventHandler)
}

func parsePaceSpec(pace string) ([]paceSpec, error) {
	paceSpecArray := strings.Split(pace, ",")
	pacerSpecs := make([]paceSpec, 0)

	for _, p := range paceSpecArray {
		ps := strings.Split(p, ":")
		rps, err := strconv.Atoi(ps[0])
		if err != nil {
			return nil, fmt.Errorf("invalid format %q: %v", ps, err)
		}
		duration := defaultDuration

		if len(ps) == 2 {
			durationSec, err := strconv.Atoi(ps[1])
			if err != nil {
				return nil, fmt.Errorf("invalid format %q: %v", ps, err)
			}
			duration = time.Second * time.Duration(durationSec)
		}

		pacerSpecs = append(pacerSpecs, paceSpec{rps, duration})
	}

	return pacerSpecs, nil
}

func (ex *senderReceiverExecutor) warmup(ctx context.Context, warmupSeconds uint) error {
	ignoreEvents := func(event cloudevents.Event) {}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if err := ex.startCloudEventsReceiver(ctx, ignoreEvents); err != nil {
			fatalf("Failed to start CloudEvents receiver: %v", err)
		}
	}()
	defer cancel()

	printf("Started CloudEvents receiver for warmup")

	// Warmup slowly increasing the rps
	pacer, err := pkgpacers.NewSteadyUp(
		vegeta.Rate{
			Freq: 10,
			Per:  time.Second,
		},
		vegeta.Rate{
			Freq: warmupRps,
			Per:  time.Second,
		},
		time.Duration(warmupSeconds/2)*time.Second,
	)
	if err != nil {
		return fmt.Errorf("failed to create pacer: %v", err)
	}

	printf("Pacer configured for warmup: 10 rps to %d rps for %d secs", warmupRps, warmupSeconds)

	printf("Starting warmup")

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, warmupEventType, defaultEventSource).VegetaTargeter()

	vegetaResults := vegeta.NewAttacker(
		vegeta.Workers(workers),
	).Attack(targeter, pacer, time.Duration(warmupSeconds)*time.Second, warmupEventType+"-attack")

	for range vegetaResults {
	}

	// give the channel a chance to drain the events it may still have enqueued
	time.Sleep(5 * time.Second)

	return nil
}

// processVegetaResult processes the results from the Vegeta attackers.
func (ex *senderReceiverExecutor) closeChannels() {
	printf("All requests sent")

	close(ex.sentCh)
	close(ex.acceptedCh)
	close(ex.failedCh)

	// assume all responses are received after a certain time
	time.Sleep(8 * time.Second)
	close(ex.receivedCh)

	printf("All channels closed")
}

// processReceiveEvent processes the event received by the CloudEvents receiver.
func (ex *senderReceiverExecutor) processReceiveEvent(event cloudevents.Event) {
	ex.receivedCh <- receivedState{eventId: event.ID(), at: ptypes.TimestampNow()}
}

// processEvents keeps a record of all events (sent, accepted, failed, received).
func (ex *senderReceiverExecutor) processEvents() {
	for {
		select {

		case e, ok := <-ex.sentCh:
			if !ok {
				continue
			}
			ex.sentEvents.Events[e.eventId] = e.at

		case e, ok := <-ex.acceptedCh:
			if !ok {
				continue
			}

			_, sent := ex.sentEvents.Events[e.eventId]
			if !sent {
				// Send timestamp not yet recorded, reenqueue
				ex.acceptedCh <- e
				continue
			}

			ex.acceptedEvents.Events[e.eventId] = e.at

		case e, ok := <-ex.failedCh:
			if !ok {
				continue
			}

			_, sent := ex.sentEvents.Events[e.eventId]
			if !sent {
				// Send timestamp not yet recorded, reenqueue
				ex.failedCh <- e
				continue
			}

			ex.failedEvents.Events[e.eventId] = e.at

		case e, ok := <-ex.receivedCh:
			if !ok {
				return
			}

			ex.receivedEvents.Events[e.eventId] = e.at
		}
	}
}

func (ex *senderReceiverExecutor) sendEventsRecordList(rl *pb.EventsRecordList) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := ex.aggCli.RecordEvents(ctx, rl)
	return err
}

// newCloudEventsClient returns a CloudEvents client.
func newCloudEventsClient(sinkURL string) (ceclient.Client, error) {
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	return cloudevents.NewClient(t)
}

func eventsSource() string {
	if pn := os.Getenv(podNameEnvVar); pn != "" {
		return pn
	}
	return defaultEventSource
}

func testNamespace() string {
	if pn := os.Getenv(podNamespaceEnvVar); pn != "" {
		return pn
	}
	return defaultTestNamespace
}

func waitForPods(namespace string) error {
	c, err := pkgtest.NewKubeClient("", "")
	if err != nil {
		return err
	}
	return pkgtest.WaitForAllPodsRunning(c, namespace)
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
		conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if !free {
		fatalf("Timeout waiting for TCP port %s to become available")
	}
}
