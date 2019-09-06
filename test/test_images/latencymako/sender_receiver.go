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
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	ceclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	vegeta "github.com/tsenart/vegeta/lib"

	"knative.dev/eventing/test/common"
	pb "knative.dev/eventing/test/test_images/latencymako/event_state"
	pkgtest "knative.dev/pkg/test"
	pkgpacers "knative.dev/pkg/test/vegeta/pacers"
)

const (
	defaultEventType      = "perf-test-event-type"
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
		vegeta.MaxWorkers(workers),
	)

	return executor
}

type requestInterceptor struct {
	before func(*http.Request)
	after  func(*http.Request, *http.Response, error)
}

func (r requestInterceptor) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.before != nil {
		r.before(request)
	}
	res, err := http.DefaultTransport.RoundTrip(request)
	if r.after != nil {
		r.after(request, res, err)
	}
	return res, err
}

func (ex *senderReceiverExecutor) Run(ctx context.Context) {
	// --- Warmup phase

	printf("--- BEGIN WARMUP ---")
	if warmupSeconds > 0 {
		if err := ex.warmup(warmupSeconds); err != nil {
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

	cancelCeReceiver, err := ex.startCloudEventsReceiver(ex.processReceiveEvent)
	if err != nil {
		fatalf("Failed to start CloudEvents receiver: %v", err)
	}
	defer cancelCeReceiver()

	// Start the events processor
	printf("Starting events processor")
	go ex.processEvents()

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, eventsSource(), "binary").VegetaTargeter()

	pacers := make([]vegeta.Pacer, len(ex.pacerSpecs))
	durations := make([]time.Duration, len(ex.pacerSpecs))
	var totalBenchmarkDuration time.Duration = 0

	for i, ps := range ex.pacerSpecs {
		pacers[i] = vegeta.ConstantPacer{ps.rps, time.Second}
		durations[i] = ps.duration
		printf("%d° pace: %d rps for %v seconds", i+1, ps.rps, ps.duration)
		totalBenchmarkDuration = totalBenchmarkDuration + ps.duration
	}

	printf("Total benchmark duration: %v", totalBenchmarkDuration.Seconds())

	combinedPacer, err := pkgpacers.NewCombined(pacers, durations)
	if err != nil {
		fatalf("Failed to setup combined pacer: %v", err)
	}

	printf("Starting benchmark")

	vegetaResults := ex.attacker.Attack(targeter, combinedPacer, totalBenchmarkDuration, defaultEventType+"-attack")

	// doneCh is closed as soon as all results have been processed
	doneCh := make(chan struct{})
	go ex.processVegetaResult(vegetaResults, doneCh)
	<-doneCh

	printf("---- END BENCHMARK ----")

	printf("Sending collected data to the aggregator")

	printf("%-15s: %d", "Sent count", len(ex.sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(ex.acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(ex.failedEvents.Events))
	printf("%-15s: %d", "Received count", len(ex.receivedEvents.Events))

	err = ex.sendEventsRecordList(&pb.EventsRecordList{Items: []*pb.EventsRecord{
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

func (ex *senderReceiverExecutor) startCloudEventsReceiver(eventHandler receiverFn) (context.CancelFunc, error) {
	cli, err := newCloudEventsClient(sinkURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudEvents client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := cli.StartReceiver(ctx, eventHandler); err != nil {
			fatalf("Failed to start CloudEvents receiver: %v", err)
		}
	}()

	return cancel, nil
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

func (ex *senderReceiverExecutor) warmup(warmupSeconds uint) error {
	cancelCeReceiver, err := ex.startCloudEventsReceiver(func(event cloudevents.Event) {})
	if err != nil {
		return fmt.Errorf("failed to start CloudEvents receiver: %v", err)
	}
	defer cancelCeReceiver()

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

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, defaultEventSource, "binary").VegetaTargeter()

	vegetaResults := vegeta.NewAttacker(
		vegeta.Workers(workers),
		vegeta.MaxWorkers(workers),
	).Attack(targeter, pacer, time.Duration(warmupSeconds)*time.Second, defaultEventType+"-warmup")

	for range vegetaResults {
	}

	// give the channel a chance to drain the events it may still have enqueued
	time.Sleep(5 * time.Second)

	return nil
}

// processVegetaResult processes the results from the Vegeta attackers.
func (ex *senderReceiverExecutor) processVegetaResult(vegetaResults <-chan *vegeta.Result, doneCh chan<- struct{}) {
	// Discard all vegeta results and wait the end of this channel
	for range vegetaResults {
	}

	printf("All requests sent")

	close(ex.sentCh)
	close(ex.acceptedCh)
	close(ex.failedCh)

	// assume all responses are received after a certain time
	time.Sleep(8 * time.Second)
	close(ex.receivedCh)

	printf("All channels closed")

	close(doneCh)
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
