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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/knative/eventing/test/performance"
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
	eventMap           sync.Map
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
	sentTime int64
	latency  int64
	status   eventStatus
}

func init() {
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
	flag.IntVar(&msgSize, "msg-size", 100, "The size of each message we want to send. Generate random strings to avoid caching.")
	flag.IntVar(&eventNum, "event-count", 10, "The number of events we want to send.")
	flag.IntVar(&timeout, "timeout", 30, "Timeout in seconds. If we do not receive a message back within a limited time, we consider it to be dropped.")
	flag.Float64Var(&errorRateThreshold, "error-rate-threshold", 0.1, "Rate of error event deliveries we allow. We fail the test if the error rate crosses the threshold.")
	flag.StringVar(&encoding, "encoding", "binary", "The encoding of the cloud event, one of(binary, structured).")
}

func receivedEvent(event cloudevents.Event) {
	eventID := event.ID()
	if eventState, ok := eventMap.Load(eventID); ok {
		state := eventState.(*state)
		switch state.status {
		// if the status is sent, calculate the latency
		case sent:
			now := time.Now().UTC().UnixNano()
			state.latency = now - state.sentTime
			state.status = received
		// if the status is received or duplicated, consider the event to be duplicate
		case received, duplicated:
			state.status = duplicated
		// for now, we do not consider any other statuses
		default:
		}
	} else {
		failTest(fmt.Sprintf("unexpected event received: %v\n", eventID))
	}
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

	// sleep 5 seconds before sending the events
	time.Sleep(5 * time.Second)

	// start sending events to the sink
	for seq := 0; seq < eventNum; seq++ {
		seqStr := strconv.Itoa(seq)
		payload := map[string]string{"msg": generateRandString(msgSize)}
		event := cloudevents.NewEvent()
		event.SetID(seqStr)
		event.SetType(defaultEventType)
		event.SetSource(defaultEventSource)
		if err := event.SetData(payload); err != nil {
			failTest(fmt.Sprintf("failed to set data: %v", err))
		}

		go func() {
			eventMap.Store(seqStr, &state{
				sentTime: time.Now().UTC().UnixNano(),
				status:   sent,
			})
			if _, err := c.Send(context.Background(), event); err != nil {
				eventMap.Store(seqStr, &state{
					status: undelivered,
				})
			}
		}()
	}

	// wait for all event states to be ready, that is either received or erroneous
	waitForEventStatesReady()

	// export result for this test
	exportTestResult()
}

func waitForEventStatesReady() {
	wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		eventStatesReady := true
		for seq := 0; seq < eventNum; seq++ {
			seqStr := strconv.Itoa(seq)
			eventState, ok := eventMap.Load(seqStr)
			// if the event does not exist in eventMap, it means it hasn't been sent
			if !ok {
				eventStatesReady = false
				break
			} else {
				state := eventState.(*state)
				// only check events in sent status
				if state.status == sent {
					now := time.Now().UTC().UnixNano()
					if float64(now-state.sentTime)/float64(1e9) > float64(timeout) {
						state.status = dropped
					} else {
						eventStatesReady = false
						break
					}
				}
			}
		}
		return eventStatesReady, nil
	})
}

func exportTestResult() {
	// number of abnormal event deliveries
	var errorCount int
	var latencies = make([]int64, 0)
	eventMap.Range(func(_ interface{}, eventState interface{}) bool {
		state := eventState.(*state)
		switch state.status {
		case received:
			latencies = append(latencies, state.latency)
		case dropped, duplicated, undelivered:
			errorCount++
		default:
			errorCount++
		}
		return true
	})

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
