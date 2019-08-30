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
	"log"
	"math/rand"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"golang.org/x/sync/errgroup"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/test/mako"
)

const (
	defaultEventType   = "perf-test-event-type"
	defaultEventSource = "perf-test-event-source"
)

// flags for the image
var (
	benchmark    string
	sinkURL      string
	msgSize      int
	eventNum     int
	timeout      int
	encoding     string
	eventTimeMap map[string]chan time.Time
	resultCh     chan state
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

// state saves the data that is used to generate the metrics
type state struct {
	status eventStatus
}

func init() {
	flag.StringVar(&sinkURL, "sink", "http://default-broker", "The sink URL for the event destination.")
	flag.IntVar(&msgSize, "msg-size", 100, "The size of each message we want to send. Generate random strings to avoid caching.")
	flag.IntVar(&eventNum, "event-count", 10, "The number of events we want to send.")
	flag.IntVar(&timeout, "timeout", 30, "Timeout in seconds. If we do not receive a message back within a limited time, we consider it to be dropped.")
	flag.StringVar(&encoding, "encoding", "binary", "The encoding of the cloud event, one of(binary, structured).")

	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// generateRandString returns a random string with the given length.
func generateRandString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func main() {
	// parse the command line flags
	flag.Parse()

	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	// We cron every 5 minutes, so make sure that we don't severely overrun to
	// limit how noisy a neighbor we can be.
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()

	// Use the benchmark key created
	ctx, q, qclose, err := mako.Setup(ctx)
	if err != nil {
		log.Fatalf("Failed to setup mako: %v", err)
	}

	// Use a fresh context here so that our RPC to terminate the sidecar
	// isn't subject to our timeout (or we won't shut it down when we time out)
	defer qclose(context.Background())

	// Wrap fatalf in a helper or our sidecar will live forever.
	fatalf := func(f string, args ...interface{}) {
		qclose(context.Background())
		log.Fatalf(f, args...)
	}

	// get encoding
	var encodingOption http.Option
	switch encoding {
	case "binary":
		encodingOption = cloudevents.WithBinaryEncoding()
	case "structured":
		encodingOption = cloudevents.WithStructuredEncoding()
	default:
		fatalf("unsupported encoding option: %q\n", encoding)
	}

	// create cloudevents client
	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		encodingOption,
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

	// start a server to receive the events
	go c.StartReceiver(context.Background(), receivedEvent)

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
			_, err := c.Send(ctx, event)
			if err != nil {
				if qerr := q.AddError(mako.XTime(sendTime), err.Error()); qerr != nil {
					log.Printf("ERROR AddError: %v", qerr)
				}
				resultCh <- state{status: undelivered}
			} else {
				elapsed := time.Now().Sub(sendTime)
				if qerr := q.AddSamplePoint(mako.XTime(sendTime), map[string]float64{"pl": elapsed.Seconds()}); qerr != nil {
					log.Printf("ERROR AddSamplePoint: %v", qerr)
				}
			}

			if timeCh, ok := eventTimeMap[seqStr]; ok {
				select {
				// if the event is received, calculate the delay
				case receivedTime := <-timeCh:
					latency := receivedTime.Sub(sendTime)
					if qerr := q.AddSamplePoint(mako.XTime(sendTime), map[string]float64{"dl": latency.Seconds()}); qerr != nil {
						log.Printf("ERROR AddSamplePoint: %v", qerr)
					}
					resultCh <- state{status: received}
				// if the event is not received before timeout, consider it to be dropped
				case <-ctx.Done():
					if qerr := q.AddError(mako.XTime(sendTime), "Timeout waiting for delivery"); qerr != nil {
						log.Printf("ERROR AddError: %v", qerr)
					}
					resultCh <- state{status: dropped}
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		fatalf("unexpected error happened when sending events: %v", err)
	}

	close(resultCh)

	// count errors
	var publishErrorCount int
	var deliverErrorCount int
	for eventState := range resultCh {
		switch eventState.status {
		case dropped:
			deliverErrorCount++
		case undelivered:
			publishErrorCount++
		}
	}

	// publish error counts as aggregate metrics
	q.AddRunAggregate("pe", float64(publishErrorCount))
	q.AddRunAggregate("de", float64(deliverErrorCount))

	out, err := q.Store()
	if err != nil {
		fatalf("q.Store error: %v: %v", out, err)
	}
}

func receivedEvent(event cloudevents.Event) {
	eventID := event.ID()
	if timeCh, ok := eventTimeMap[eventID]; ok {
		timeCh <- time.Now()
	}
}
