/*
Copyright 2020 The Knative Authors

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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/test/lib/recordevents"
)

type envConfig struct {
	SenderName string `envconfig:"POD_NAME" default:"sender-default" required:"true"`

	// Sink url for the message destination
	Sink string `envconfig:"SINK" required:"true"`

	// InputEvent json encoded
	InputEvent string `envconfig:"INPUT_EVENT" required:"false"`

	// InputHeaders to send (this overrides any event provided input)
	InputHeaders map[string]string `envconfig:"INPUT_HEADERS" required:"false"`

	// InputBody to send (this overrides any event provided input)
	InputBody string `envconfig:"INPUT_BODY" required:"false"`

	// The encoding of the cloud event: [binary, structured].
	EventEncoding string `envconfig:"EVENT_ENCODING" default:"binary" required:"false"`

	// The number of seconds between messages.
	Period int `envconfig:"PERIOD" default:"5" required:"false"`

	// The number of seconds to wait before starting sending the first message
	Delay int `envconfig:"DELAY" default:"5" required:"false"`

	// The number of messages to attempt to send. 0 for unlimited.
	MaxMessages int `envconfig:"MAX_MESSAGES" default:"1" required:"false"`

	// Should tracing be added to events sent.
	AddTracing bool `envconfig:"ADD_TRACING" default:"false" required:"false"`

	// Should add extension 'sequence' identifying the sequence number.
	AddSequence bool `envconfig:"ADD_SEQUENCE" default:"false" required:"false"`

	// Override the event id with an incremental id.
	IncrementalId bool `envconfig:"INCREMENTAL_ID" default:"false" required:"false"`
}

func Start(ctx context.Context, logs *recordevents.EventLogs) error {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		return fmt.Errorf("failed to process env var. %w", err)
	}

	logging.FromContext(ctx).Infof("Sender environment configuration: %+v", env)

	if env.InputEvent == "" && env.InputBody == "" && len(env.InputHeaders) == 0 {
		return fmt.Errorf("input values not provided")
	}

	flag.Parse()
	period := time.Duration(env.Period) * time.Second
	delay := time.Duration(env.Delay) * time.Second

	if delay > 0 {
		logging.FromContext(ctx).Info("will sleep for ", delay)
		time.Sleep(delay)
		logging.FromContext(ctx).Info("awake, continuing")
	}

	switch env.EventEncoding {
	case "binary":
		ctx = cloudevents.WithEncodingBinary(ctx)
	case "structured":
		ctx = cloudevents.WithEncodingStructured(ctx)
	default:
		return fmt.Errorf("unsupported encoding option: %q", env.EventEncoding)
	}

	httpClient := &nethttp.Client{}
	if env.AddTracing {
		httpClient.Transport = &ochttp.Transport{
			Base:        nethttp.DefaultTransport,
			Propagation: tracecontextb3.TraceContextEgress,
		}
	}

	var baseEvent *cloudevents.Event
	if env.InputEvent != "" {
		if err := json.Unmarshal([]byte(env.InputEvent), &baseEvent); err != nil {
			return fmt.Errorf("unable to unmarshal the event from json: %w", err)
		}
	}

	sequence := 0

	ticker := time.NewTicker(period)
	for {
		req, err := nethttp.NewRequest(nethttp.MethodPost, env.Sink, nil)
		if err != nil {
			logging.FromContext(ctx).Error("Cannot create the request: ", err)
			return err
		}

		var event *cloudevents.Event
		if baseEvent != nil {
			e := baseEvent.Clone()
			event = &e

			sequence++
			if env.AddSequence {
				event.SetExtension("sequence", sequence)
			}
			if env.IncrementalId {
				event.SetID(strconv.Itoa(sequence))
			}

			logging.FromContext(ctx).Info("I'm going to send\n", event)

			err := cehttp.WriteRequest(ctx, binding.ToMessage(event), req)
			if err != nil {
				logging.FromContext(ctx).Error("Cannot write the event: ", err)
				return err
			}
		}

		if len(env.InputHeaders) != 0 {
			for k, v := range env.InputHeaders {
				req.Header.Add(k, v)
			}
		}

		if env.InputBody != "" {
			req.Body = ioutil.NopCloser(bytes.NewReader([]byte(env.InputBody)))
		}

		res, err := httpClient.Do(req)

		if err != nil {
			// Publish error
			if err := logs.Vent(recordevents.EventInfo{
				Kind:     recordevents.EventSent,
				Error:    err.Error(),
				Origin:   env.SenderName,
				Observer: env.SenderName,
				Time:     time.Now(),
				Sequence: uint64(sequence),
			}); err != nil {
				return fmt.Errorf("cannot forward event info: %w", err)
			}
		} else {
			sentEventInfo := recordevents.EventInfo{
				Kind:     recordevents.EventSent,
				Event:    event,
				Origin:   env.SenderName,
				Observer: env.SenderName,
				Time:     time.Now(),
				Sequence: uint64(sequence),
			}

			sentHeaders := make(nethttp.Header)
			for k, v := range req.Header {
				if !strings.HasPrefix(k, "Ce-") {
					sentHeaders[k] = v
				}
			}
			sentEventInfo.HTTPHeaders = sentHeaders

			if env.InputBody != "" {
				sentEventInfo.Body = []byte(env.InputBody)
			}

			// Publish sent event info
			if err := logs.Vent(sentEventInfo); err != nil {
				return fmt.Errorf("cannot forward event info: %w", err)
			}

			// Now let's figure out what's inside the response
			responseMessage := cehttp.NewMessageFromHttpResponse(res)

			responseInfo := recordevents.EventInfo{
				Kind:        recordevents.EventResponse,
				HTTPHeaders: res.Header,
				Origin:      env.Sink,
				Observer:    env.SenderName,
				Time:        time.Now(),
				Sequence:    uint64(sequence),
				StatusCode:  res.StatusCode,
			}
			if responseMessage.ReadEncoding() == binding.EncodingUnknown {
				body, err := ioutil.ReadAll(res.Body)

				if err != nil {
					responseInfo.Error = err.Error()
				} else {
					responseInfo.Body = body
				}
			} else {
				responseEvent, err := binding.ToEvent(ctx, responseMessage)
				if err != nil {
					responseInfo.Error = err.Error()
				} else {
					responseInfo.Event = responseEvent
				}
			}

			// Vent the response info
			if err := logs.Vent(responseInfo); err != nil {
				return fmt.Errorf("cannot forward event info: %w", err)
			}
		}

		// Wait for next tick
		<-ticker.C
		// Only send a limited number of messages.
		if env.MaxMessages != 0 && env.MaxMessages == sequence {
			return nil
		}

		// Check if ctx is done before the next loop
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Infof("Canceled sending messages because context was closed")
			return nil
		default:
		}
	}
}
