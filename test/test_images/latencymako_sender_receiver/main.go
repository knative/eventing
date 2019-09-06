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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	vegeta "github.com/tsenart/vegeta/lib"

	"knative.dev/eventing/test/common"
	pb "knative.dev/eventing/test/test_images/latencymako_aggregator/event_state"
	pkgtest "knative.dev/pkg/test"
	pkgpacers "knative.dev/pkg/test/vegeta/pacers"
)

const (
	defaultEventType     = "perf-test-event-type"
	defaultEventSource   = "perf-test-event-source"
	warmupRps            = 100
	defaultDuration      = 10 * time.Second
	defaultTestNamespace = "default"
)

// flags for the image
var (
	sinkURL       string
	aggregAddr    string
	msgSize       int
	workers       uint64
	paceFlag      string
	warmupSeconds uint
	verbose       bool
)

// environment variables consumed by the test
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

// state channels
var (
	sentCh     chan sentState
	acceptedCh chan acceptedState
	failedCh   chan failedState
	receivedCh chan receivedState
)

// events recording maps
var (
	sentEvents     = &pb.EventsRecord{Type: pb.EventsRecord_SENT}
	acceptedEvents = &pb.EventsRecord{Type: pb.EventsRecord_ACCEPTED}
	failedEvents   = &pb.EventsRecord{Type: pb.EventsRecord_FAILED}
	receivedEvents = &pb.EventsRecord{Type: pb.EventsRecord_RECEIVED}
)

var fatalf = log.Fatalf

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

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
	flag.StringVar(&aggregAddr, "aggregator", "", "The aggregator address for sending events records.")
	flag.IntVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.UintVar(&warmupSeconds, "warmup", 10, "Duration in seconds of warmup phase. During warmup latencies are not recorded. 0 means no warmup")
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")
	flag.Uint64Var(&workers, "workers", 1, "Number of vegeta workers")
}

type paceSpec struct {
	rps      int
	duration time.Duration
}

func main() {
	// parse the command line flags
	flag.Parse()

	if paceFlag == "" {
		fatalf("pace not set!")
	}

	if sinkURL == "" {
		fatalf("sink not set!")
	}

	if aggregAddr == "" {
		fatalf("aggregator not set!")
	}

	pacerSpecs, err := parsePaceSpec()
	if err != nil {
		fatalf("Failed to parse pace spec: %v", err)
	}

	// wait until all pods are ready (channel, consumers)
	ns := testNamespace()
	printf("Waiting for all Pods to be ready in namespace %s", ns)
	if err := waitForPods(ns); err != nil {
		fatalf("Timeout waiting for Pods readiness in namespace %s: %v", ns, err)
	}

	// --- Warmup phase

	printf("--- BEGIN WARMUP ---")
	if warmupSeconds > 0 {
		warmup(warmupSeconds)
	} else {
		printf("Warmup skipped")
	}
	printf("---- END WARMUP ----")

	printf("--- BEGIN BENCHMARK ---")

	// --- Allocate channels

	printf("Configuring channels")

	// We need those estimates to allocate memory before benchmark starts
	var estimatedNumberOfMessagesInsideAChannel uint64
	var estimatedNumberOfTotalMessages uint64

	for _, pacer := range pacerSpecs {
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
	sentCh = make(chan sentState, estimatedNumberOfMessagesInsideAChannel)
	acceptedCh = make(chan acceptedState, estimatedNumberOfMessagesInsideAChannel)
	failedCh = make(chan failedState, estimatedNumberOfMessagesInsideAChannel)
	receivedCh = make(chan receivedState, estimatedNumberOfMessagesInsideAChannel)

	// Small note: receivedCh depends on receive thpt and not send thpt but we
	// don't care since this is a pessimistic estimate and receive thpt < send thpt

	sentEvents.Events = make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages)
	acceptedEvents.Events = make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages)
	failedEvents.Events = make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages)
	receivedEvents.Events = make(map[string]*timestamp.Timestamp, estimatedNumberOfTotalMessages)

	// Start the events receiver
	printf("Starting CloudEvents receiver")
	startCloudEventsReceiver(processReceiveEvent)

	printf("Starting latency processor")

	// Start the events processor
	go processEvents()

	// set events defaults
	eventSrc := eventsSource()

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, eventSrc, "binary").VegetaTargeter()

	pacers := make([]vegeta.Pacer, len(pacerSpecs))
	durations := make([]time.Duration, len(pacerSpecs))
	var totalBenchmarkDuration time.Duration = 0

	for i, ps := range pacerSpecs {
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

	client := http.Client{Transport: requestInterceptor{
		before: func(request *http.Request) {
			id := request.Header.Get("Ce-Id")
			sentCh <- sentState{eventId: id, at: ptypes.TimestampNow()}
		},
		after: func(request *http.Request, response *http.Response, e error) {
			id := request.Header.Get("Ce-Id")
			t := ptypes.TimestampNow()
			if e != nil || response.StatusCode < 200 || response.StatusCode > 300 {
				failedCh <- failedState{eventId: id, at: t}
			} else {
				acceptedCh <- acceptedState{eventId: id, at: t}
			}
		},
	}}

	printf("Starting benchmark")

	vegetaResults := vegeta.NewAttacker(
		vegeta.Client(&client),
		vegeta.Workers(workers),
		vegeta.MaxWorkers(workers),
	).Attack(targeter, combinedPacer, totalBenchmarkDuration, defaultEventType+"-attack")

	// doneCh is closed as soon as all results have been processed
	doneCh := make(chan struct{})
	go processVegetaResult(vegetaResults, doneCh)
	<-doneCh

	printf("---- END BENCHMARK ----")

	printf("Sending collected data to the aggregator")

	printf("%-15s: %d", "Sent count", len(sentEvents.Events))
	printf("%-15s: %d", "Accepted count", len(acceptedEvents.Events))
	printf("%-15s: %d", "Failed count", len(failedEvents.Events))
	printf("%-15s: %d", "Received count", len(receivedEvents.Events))

	// Create a connection to the aggregator
	c, connclose := aggregatorClient()
	defer connclose()

	err = sendEventsRecordList(c, &pb.EventsRecordList{Items: []*pb.EventsRecord{
		sentEvents,
		acceptedEvents,
		failedEvents,
		receivedEvents,
	}})
	if err != nil {
		fatalf("Failed to send events record: %v", err)
	}
}

func parsePaceSpec() ([]paceSpec, error) {
	paceSpecArray := strings.Split(paceFlag, ",")
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

func warmup(warmupSeconds uint) {
	cancelCeReceiver := startCloudEventsReceiver(func(event cloudevents.Event) {})

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
		fatalf("Failed to create pacer: %v", err)
	}

	printf("Pacer configured for warmup: 10 rps to %d rps for %d secs", warmupRps, warmupSeconds)

	printf("Starting warmup")

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, defaultEventSource, "binary").VegetaTargeter()

	vegetaResults := vegeta.NewAttacker(
		vegeta.Workers(workers),
		vegeta.MaxWorkers(workers),
	).Attack(targeter, pacer, time.Duration(warmupSeconds)*time.Second, defaultEventType+"-warmup")

	for _ = range vegetaResults {
	}

	// give the channel a chance to drain the events it may still have enqueued
	time.Sleep(5 * time.Second)

	cancelCeReceiver()
}

func startCloudEventsReceiver(eventHandler func(event cloudevents.Event)) context.CancelFunc {
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		fatalf("Failed to create transport: %v", err)
	}
	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go c.StartReceiver(ctx, eventHandler)

	return cancel
}

func processVegetaResult(vegetaResults <-chan *vegeta.Result, doneCh chan<- struct{}) {
	// Discard all vegeta results and wait the end of this channel
	for _ = range vegetaResults {
	}

	printf("All requests sent")

	close(sentCh)
	close(acceptedCh)

	// assume all responses are received after a certain time
	time.Sleep(8 * time.Second)
	close(receivedCh)

	printf("All channels closed")

	close(doneCh)
}

func processReceiveEvent(event cloudevents.Event) {
	et, err := ptypes.TimestampProto(event.Time())
	if err != nil {
		fatalf("Received an invalid timestamp in event %q: %v", event.ID(), err)
	}

	receivedCh <- receivedState{eventId: event.ID(), at: et}
}

// processEvents keeps a record of all events (sent, accepted, failed, received).
func processEvents() {
	for {
		select {

		case e, ok := <-sentCh:
			if !ok {
				continue
			}
			sentEvents.Events[e.eventId] = e.at

		case e, ok := <-acceptedCh:
			if !ok {
				continue
			}

			_, sent := sentEvents.Events[e.eventId]
			if !sent {
				// Send timestamp not yet recorded, reenqueue
				acceptedCh <- e
				continue
			}

			acceptedEvents.Events[e.eventId] = e.at

		case e, ok := <-failedCh:
			if !ok {
				continue
			}

			_, sent := sentEvents.Events[e.eventId]
			if !sent {
				// Send timestamp not yet recorded, reenqueue
				failedCh <- e
				continue
			}

			failedEvents.Events[e.eventId] = e.at

		case e, ok := <-receivedCh:
			if !ok {
				return
			}

			receivedEvents.Events[e.eventId] = e.at
		}
	}
}

func aggregatorClient() (cli pb.EventsRecorderClient, closeConnFn func() error) {
	conn, err := grpc.Dial(aggregAddr, grpc.WithInsecure())
	if err != nil {
		fatalf("Failed to connect to the aggregator: %v", err)
	}

	return pb.NewEventsRecorderClient(conn), conn.Close
}

func sendEventsRecordList(cli pb.EventsRecorderClient, rl *pb.EventsRecordList) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := cli.RecordEvents(ctx, rl)
	return err
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
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
