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
	"github.com/google/mako/go/quickstore"
	"knative.dev/eventing/test/common"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	vegeta "github.com/tsenart/vegeta/lib"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/test/mako"
	pkgpacers "knative.dev/pkg/test/vegeta/pacers"
)

const (
	defaultEventType   = "perf-test-event-type"
	defaultEventSource = "perf-test-event-source"
	warmupRps          = 100
	defaultDuration    = 10 * time.Second
)

// flags for the image
var (
	sinkURL       string
	msgSize       int
	workers       uint64
	sentCh        chan sentState
	deliveredCh   chan deliveredState
	receivedCh    chan receivedState
	resultCh      chan eventStatus
	paceFlag      string
	warmupSeconds uint
	verbose       bool
	fatalf        = log.Fatalf
)

// eventStatus is status of the event delivery.
type eventStatus int

const (
	sent eventStatus = iota
	received
	undelivered
	dropped
	duplicated // TODO currently unused
	corrupted  // TODO(Fredy-Z): corrupted status is not being used now
)

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

type state struct {
	eventId uint64
	failed  bool
	at      time.Time
}

type sentState state
type deliveredState state
type receivedState state

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
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

	pacerSpecs, err := parsePaceSpec()
	if err != nil {
		fatalf("%+v", err)
	}

	// --- Configure mako

	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	// Use the benchmark key created
	ctx, q, qclose, err := mako.Setup(ctx)
	if err != nil {
		log.Fatalf("Failed to setup mako: %v", err)
	}

	// Use a fresh context here so that our RPC to terminate the sidecar
	// isn't subject to our timeout (or we won't shut it down when we time out)
	defer qclose(context.Background())

	// Wrap fatalf in a helper or our sidecar will live forever.
	fatalf = func(f string, args ...interface{}) {
		qclose(context.Background())
		log.Fatalf(f, args...)
	}

	printf("Mako configured")

	// --- Warmup phase

	if warmupSeconds > 0 {
		warmup(warmupSeconds)
	} else {
		printf("Warmup skipped")
	}

	printf("--- BENCHMARK ---")
	printf("Configuring channels")

	// --- Allocate channels

	// We need those estimates to allocate memory before benchmark starts
	var estimatedNumberOfMessagesInsideAChannel uint64
	var estimatedNumberOfTotalMessages uint64

	for _, pacer := range pacerSpecs {
		totalMessages := uint64(pacer.rps * int(pacer.duration.Seconds()))
		// Add a bit more, just to be sure that we don't allocate
		totalMessages = totalMessages + uint64(float64(totalMessages)*0.1)
		// Queueing theory stuff: Given our channels can process 50 rps, queueLength = arrival rps / 50 rps = pacer.rps / 50
		queueLength := uint64(pacer.rps / 50)
		if queueLength < 10 {
			queueLength = 10
		}
		estimatedNumberOfTotalMessages = estimatedNumberOfTotalMessages + totalMessages
		if queueLength > estimatedNumberOfMessagesInsideAChannel {
			estimatedNumberOfMessagesInsideAChannel = queueLength
		}
	}

	printf("Estimated channel size: %v", estimatedNumberOfMessagesInsideAChannel)
	printf("Estimated total messages: %v", estimatedNumberOfTotalMessages)

	// Create all channels
	sentCh = make(chan sentState, estimatedNumberOfMessagesInsideAChannel)
	deliveredCh = make(chan deliveredState, estimatedNumberOfMessagesInsideAChannel)
	receivedCh = make(chan receivedState, estimatedNumberOfMessagesInsideAChannel)
	resultCh = make(chan eventStatus, estimatedNumberOfMessagesInsideAChannel)

	// Small note: both receivedCh and resultCh depends on receive thpt and not send thpt
	// but we don't care since this is a pessimistic estimate and receive thpt < send thpt

	printf("Starting CloudEvents receiver")

	// Start the events receiver
	startCloudEventsReceiver(processReceiveEvent)

	printf("Starting latency processor")

	// Start the goroutine that will process the latencies and publish the data points to mako
	go processLatencies(q, estimatedNumberOfTotalMessages)

	if warmupSeconds > 0 {
		// Just let the channel relax a bit
		time.Sleep(5 * time.Second)
	} else {
		// sleep 30 seconds before sending the events
		// TODO(Fredy-Z): this is a bit hacky, as ideally, we need to wait for the Trigger/Subscription that uses it as a
		//                Subscriber to become ready before sending the events, but we don't have a way to coordinate between them.
		time.Sleep(30 * time.Second)
	}

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, defaultEventSource, "binary").VegetaTargeter()

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

	client := http.Client{Transport: requestInterceptor{before: func(request *http.Request) {
		id, _ := strconv.ParseUint(request.Header["Ce-Id"][0], 10, 64)
		sentCh <- sentState{eventId: id, at: time.Now()}
	}, after: func(request *http.Request, response *http.Response, e error) {
		id, _ := strconv.ParseUint(request.Header["Ce-Id"][0], 10, 64)
		if e != nil || response.StatusCode < 200 || response.StatusCode > 300 {
			deliveredCh <- deliveredState{eventId: id, failed: true}
		} else {
			deliveredCh <- deliveredState{eventId: id, at: time.Now()}
		}
	}}}

	printf("Starting benchmark")

	vegetaResults := vegeta.NewAttacker(
		vegeta.Client(&client),
		vegeta.Workers(workers),
		vegeta.MaxWorkers(workers),
	).Attack(targeter, combinedPacer, totalBenchmarkDuration, defaultEventType+"-attack")

	go processVegetaResult(vegetaResults)

	// count errors
	var publishErrorCount int
	var deliverErrorCount int
	for eventState := range resultCh {
		switch eventState {
		case dropped:
			deliverErrorCount++
		case undelivered:
			publishErrorCount++
		}
	}

	printf("Publish error count: %v", publishErrorCount)
	printf("Deliver error count: %v", deliverErrorCount)

	printf("--- END BENCHMARK ---")

	// publish error counts as aggregate metrics
	q.AddRunAggregate("pe", float64(publishErrorCount))
	q.AddRunAggregate("de", float64(deliverErrorCount))

	out, err := q.Store()
	if err != nil {
		fatalf("q.Store error: %v: %v", out, err)
	}
}

func parsePaceSpec() ([]paceSpec, error) {
	paceSpecArray := strings.Split(paceFlag, ",")
	pacerSpecs := make([]paceSpec, 0)

	for _, p := range paceSpecArray {
		ps := strings.Split(p, ":")
		rps, err := strconv.Atoi(ps[0])
		if err != nil {
			return nil, fmt.Errorf("error while parsing pace spec %v: %v", ps, err)
		}
		duration := defaultDuration

		if len(ps) == 2 {
			durationSec, err := strconv.Atoi(ps[1])
			if err != nil {
				return nil, fmt.Errorf("error while parsing pace spec %v: %v", ps, err)
			}
			duration = time.Second * time.Duration(durationSec)
		}

		pacerSpecs = append(pacerSpecs, paceSpec{rps, duration})
	}

	return pacerSpecs, nil
}

func warmup(warmupSeconds uint) {
	printf("--- WARMUP ---")

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
		fatalf("failed to create pacer: %v\n", err)
	}

	printf("Pacer configured for warmup: 10 rps to %d rps for %d secs", warmupRps, warmupSeconds)

	// sleep 30 seconds before sending the events
	// TODO(Fredy-Z): this is a bit hacky, as ideally, we need to wait for the Trigger/Subscription that uses it as a
	//                Subscriber to become ready before sending the events, but we don't have a way to coordinate between them.
	time.Sleep(30 * time.Second)

	printf("Starting warmup")

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, defaultEventType, defaultEventSource, "binary").VegetaTargeter()

	vegetaResults := vegeta.NewAttacker(
		vegeta.Workers(workers),
		vegeta.MaxWorkers(workers),
	).Attack(targeter, pacer, time.Duration(warmupSeconds)*time.Second, defaultEventType+"-warmup")

	for _ = range vegetaResults {
	}

	printf("--- END WARMUP ---")

	cancelCeReceiver()
}

func startCloudEventsReceiver(eventHandler func(event cloudevents.Event)) context.CancelFunc {
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		fatalf("failed to create transport: %v\n", err)
	}
	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		fatalf("failed to create client: %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go c.StartReceiver(ctx, eventHandler)

	return cancel
}

func processVegetaResult(vegetaResults <-chan *vegeta.Result) {
	// Discard all vegeta results and wait the end of this channel
	for _ = range vegetaResults {
	}

	printf("All requests sent")

	close(sentCh)
	close(deliveredCh)

	// Let's assume that after 5 seconds all responses are received
	time.Sleep(5 * time.Second)
	close(receivedCh)

	// Let's assume that after 3 seconds all responses are processed
	time.Sleep(3 * time.Second)
	close(resultCh)

	printf("All channels closed")
}

func processReceiveEvent(event cloudevents.Event) {
	id, _ := strconv.ParseUint(event.ID(), 10, 64)
	receivedCh <- receivedState{eventId: id, at: time.Now()}
}

func processLatencies(q *quickstore.Quickstore, mapSize uint64) {
	sentEventsMap := make(map[uint64]time.Time, mapSize)
	for {
		select {
		case s, ok := <-sentCh:
			if ok {
				sentEventsMap[s.eventId] = s.at
			}
		case d, ok := <-deliveredCh:
			if ok {
				timestampSent, ok := sentEventsMap[d.eventId]
				if ok {
					if d.failed {
						resultCh <- undelivered
						if qerr := q.AddError(mako.XTime(timestampSent), "undelivered"); qerr != nil {
							log.Printf("ERROR AddError: %v", qerr)
						}
					} else {
						sendLatency := d.at.Sub(timestampSent)
						// Uncomment to get CSV directly from this container log
						//fmt.Printf("%f,%d,\n", mako.XTime(timestampSent), sendLatency.Nanoseconds())
						// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
						if qerr := q.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"pl": sendLatency.Seconds()}); qerr != nil {
							log.Printf("ERROR AddSamplePoint: %v", qerr)
						}
					}
				} else {
					// Send timestamp still not here, reenqueue
					deliveredCh <- d
				}
			}
		case r, ok := <-receivedCh:
			if ok {
				timestampSent, ok := sentEventsMap[r.eventId]
				if ok {
					e2eLatency := r.at.Sub(timestampSent)
					// Uncomment to get CSV directly from this container log
					//fmt.Printf("%f,,%d\n", mako.XTime(timestampSent), e2eLatency.Nanoseconds())
					// TODO mako accepts float64, which imo could lead to losing some precision on local tests. It should accept int64
					if qerr := q.AddSamplePoint(mako.XTime(timestampSent), map[string]float64{"dl": e2eLatency.Seconds()}); qerr != nil {
						log.Printf("ERROR AddSamplePoint: %v", qerr)
					}
				} else {
					// Send timestamp still not here, reenqueue
					receivedCh <- r
				}
			} else {
				return
			}
		}
	}
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
}
