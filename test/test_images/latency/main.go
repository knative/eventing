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
	"os"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"golang.org/x/sync/errgroup"
	"knative.dev/eventing/test/performance"
)

const (
	defaultEventType   = "perf-test-event-type"
	defaultEventSource = "perf-test-event-source"

	// The interval and timeout used for polling in checking event states.
	pollInterval = 1 * time.Second
	pollTimeout  = 4 * time.Minute
)

// flags for the image
var (
	sinkURL            string
	msgSize            int
	eventNum           int
	errorRateThreshold float64
	timeout            int
	encoding           string
	eventTimeMap       map[string]chan time.Time
	resultCh           chan state
)

// eventStatus is status of the event, for now only if all events are in received status can the
// test be considered as PASS.
type eventStatus int

const (
	sent eventStatus = iota
	received
	undelivered
	dropped
	duplicated
	corrupted // TODO(Fredy-Z): corrupted status is not being used now
)

// state saves the data that is used to generate the metrics
type state struct {
	latency time.Duration
	status  eventStatus
}

func init() {
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
	flag.IntVar(&msgSize, "msg-size", 100, "The size of each message we want to send. Generate random strings to avoid caching.")
	flag.IntVar(&eventNum, "event-count", 10, "The number of events we want to send.")
	flag.IntVar(&timeout, "timeout", 30, "Timeout in seconds. If we do not receive a message back within a limited time, we consider it to be dropped.")
	flag.Float64Var(&errorRateThreshold, "error-rate-threshold", 0.1, "Rate of error event deliveries we allow. We fail the test if the error rate crosses the threshold.")
	flag.StringVar(&encoding, "encoding", "binary", "The encoding of the cloud event, one of(binary, structured).")
}

func main() {
	// parse the command line flags
	flag.Parse()

	// get encoding
	var encodingOption http.Option
	switch encoding {
	case "binary":
		encodingOption = cloudevents.WithBinaryEncoding()
	case "structured":
		encodingOption = cloudevents.WithStructuredEncoding()
	default:
		failTest(fmt.Sprintf("unsupported encoding option: %q\n", encoding))
	}

	// create cloudevents client
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		encodingOption,
	)
	if err != nil {
		failTest(fmt.Sprintf("failed to create transport: %v\n", err))
	}
	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		failTest(fmt.Sprintf("failed to create client: %v\n", err))
	}

	// start a server to receive the events
	go c.StartReceiver(context.Background(), receivedEvent)

	// sleep 30 seconds before sending the events
	// TODO(Fredy-Z): this is a bit hacky, as ideally, we need to wait for the Trigger/Subscription that uses it as a
	//                Subscriber to become ready before sending the events, but we don't have a way to coordinate between them.
	time.Sleep(30 * time.Second)

	// populate the eventTimeMap with channels before sending the events
	eventTimeMap = make(map[string]chan time.Time)
	for seq := 0; seq < eventNum; seq++ {
		eventTimeMap[strconv.Itoa(seq)] = make(chan time.Time)
	}
	// initialize the result channel
	resultCh = make(chan state, eventNum)

	group, _ := errgroup.WithContext(context.Background())
	// start sending events to the sink
	for seq := 0; seq < eventNum; seq++ {
		seqStr := strconv.Itoa(seq)
		group.Go(func() error {
			payload := map[string]string{"msg": generateRandString(msgSize)}
			event := cloudevents.NewEvent()
			event.SetID(seqStr)
			event.SetType(defaultEventType)
			event.SetSource(defaultEventSource)
			if err := event.SetData(payload); err != nil {
				return err
			}

			// send events with timeout
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()
			sendTime := time.Now()
			if _, _, err := c.Send(ctx, event); err != nil {
				resultCh <- state{status: undelivered}
			}
			if timeCh, ok := eventTimeMap[seqStr]; ok {
				select {
				// if the event is received, calculate the delay
				case receivedTime := <-timeCh:
					latency := receivedTime.Sub(sendTime)
					resultCh <- state{latency: latency, status: received}
				// if the event is not received before timeout, consider it to be dropped
				case <-ctx.Done():
					resultCh <- state{status: dropped}
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		failTest("unexpected error happened when sending events")
	}
	close(resultCh)

	// export result for this test
	exportTestResult()
}

func receivedEvent(event cloudevents.Event) {
	eventID := event.ID()
	if timeCh, ok := eventTimeMap[eventID]; ok {
		timeCh <- time.Now()
	}
}

func exportTestResult() {
	// number of abnormal event deliveries
	var errorCount int
	var latencies = make([]int64, 0)
	for eventState := range resultCh {
		switch eventState.status {
		case received:
			latencies = append(latencies, int64(eventState.latency))
		case dropped, duplicated, undelivered:
			errorCount++
		default:
			errorCount++
		}
	}

	// if the error rate is larger than the threshold, we consider this test to be failed
	if errorCount != 0 && float64(errorCount)/float64(eventNum) > errorRateThreshold {
		failTest(fmt.Sprintf("%d events failed to deliver", errorCount))
	}

	// use the stringbuilder to build the test result
	var builder strings.Builder
	builder.WriteString("\n")
	builder.WriteString(performance.TestResultKey + ": " + performance.TestPass)
	builder.WriteString("\n")

	// create latency metrics
	for _, perc := range []float64{0.50, 0.90, 0.99} {
		samplePercentile := float32(calculateSamplePercentile(latencies, perc)) / float32(1e9)
		name := fmt.Sprintf("p%d(s)", int(perc*100))
		builder.WriteString(fmt.Sprintf("%s: %f\n", name, samplePercentile))
	}

	log.Printf(builder.String())
}

func failTest(reason string) {
	var builder strings.Builder
	builder.WriteString("\n")
	builder.WriteString(performance.TestResultKey + ": " + performance.TestFail + "\n")
	builder.WriteString(performance.TestFailReason + ": " + reason)
	log.Fatalf(builder.String())
	os.Exit(1)
}
