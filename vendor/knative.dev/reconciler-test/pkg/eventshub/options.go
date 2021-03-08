/*
Copyright 2020 The Knative Authors

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

package eventshub

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/network"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
)

// EventsHubOption is used to define an env for the eventshub image
type EventsHubOption = func(context.Context, map[string]string) error

// StartReceiver starts the receiver in the eventshub
// This can be used together with EchoEvent, ReplyWithTransformedEvent, ReplyWithAppendedData
var StartReceiver EventsHubOption = envAdditive("EVENT_GENERATORS", "receiver")

// StartSender starts the sender in the eventshub
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSender(sinkSvc string) EventsHubOption {
	return compose(envAdditive("EVENT_GENERATORS", "sender"), func(ctx context.Context, envs map[string]string) error {
		envs["SINK"] = "http://" + network.GetServiceHostname(sinkSvc, environment.FromContext(ctx).Namespace())
		return nil
	})
}

// StartSenderToResource starts the sender in the eventshub pointing to the provided resource
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderToResource(gvr schema.GroupVersionResource, name string) EventsHubOption {
	return compose(envAdditive("EVENT_GENERATORS", "sender"), func(ctx context.Context, envs map[string]string) error {
		u, err := k8s.Address(ctx, gvr, name)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("resource %v named %s is not addressable", gvr, name)
		}
		envs["SINK"] = u.String()
		return nil
	})
}

// StartSenderURL starts the sender in the eventshub sinking to a URL.
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderURL(sink string) EventsHubOption {
	return compose(envAdditive("EVENT_GENERATORS", "sender"), func(ctx context.Context, envs map[string]string) error {
		envs["SINK"] = sink
		return nil
	})
}

// --- Receiver options

// EchoEvent is an option to let the eventshub reply with the received event
var EchoEvent EventsHubOption = envOption("REPLY", "true")

// ReplyWithTransformedEvent is an option to let the eventshub reply with the transformed event
func ReplyWithTransformedEvent(replyEventType string, replyEventSource string, replyEventData string) EventsHubOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_EVENT_TYPE", replyEventType),
		envOptionalOpt("REPLY_EVENT_SOURCE", replyEventSource),
		envOptionalOpt("REPLY_EVENT_DATA", replyEventData),
	)
}

// ReplyWithAppendedData is an option to let the eventshub reply with the transformed event with appended data
func ReplyWithAppendedData(appendData string) EventsHubOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_APPEND_DATA", appendData),
	)
}

// ResponseWaitTime defines how much the receiver has to wait before replying.
func ResponseWaitTime(delay time.Duration) EventsHubOption {
	return envDuration("RESPONSE_WAIT_TIME", delay)
}

// --- Sender options

// InitialSenderDelay defines how much the sender has to wait, when started, before start sending events.
// Note: this delay is executed before the probe sink.
func InitialSenderDelay(delay time.Duration) EventsHubOption {
	return envDuration("DELAY", delay)
}

// EnableProbeSink probes the sink with HTTP head requests up until the sink replies.
// The specified duration defines the maximum timeout to probe it, before failing.
// Note: the probe sink is executed after the initial delay
func EnableProbeSink(timeout time.Duration) EventsHubOption {
	return compose(
		envOption("PROBE_SINK", "true"),
		envDuration("PROBE_SINK_TIMEOUT", timeout),
	)
}

// DisableProbeSink will disable the probe sink feature of sender, starting sending directly events after it's started.
var DisableProbeSink = envOption("PROBE_SINK", "false")

// InputEvent is an option to provide the event to send when deploying the event sender
func InputEvent(event cloudevents.Event) EventsHubOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(ctx context.Context, envs map[string]string) error {
			return err
		}
	}
	return envOption("INPUT_EVENT", string(encodedEvent))
}

// InputEventWithEncoding is an option to provide the event to send when deploying the event sender forcing the specified encoding.
func InputEventWithEncoding(event cloudevents.Event, encoding cloudevents.Encoding) EventsHubOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(ctx context.Context, envs map[string]string) error {
			return err
		}
	}
	return compose(
		envOption("INPUT_EVENT", string(encodedEvent)),
		envOption("EVENT_ENCODING", encoding.String()),
	)
}

// InputHeader adds the following header to the sent headers.
func InputHeader(k, v string) EventsHubOption {
	return envAdditive("INPUT_HEADERS", k+":"+v)
}

// InputBody overwrites the request header with the following body.
func InputBody(b string) EventsHubOption {
	return envOption("INPUT_BODY", b)
}

// AddTracing adds tracing headers when sending events.
var AddTracing = envOption("ADD_TRACING", "true")

// AddSequence adds an extension named 'sequence' which contains the incremental number of sent events
// (similar to EnableIncrementalId, but without overwriting the id attribute).
var AddSequence = envOption("ADD_SEQUENCE", "true")

// EnableIncrementalId replaces the event id with a new incremental id for each sent event.
var EnableIncrementalId = envOption("INCREMENTAL_ID", "true")

// SendMultipleEvents defines how much events to send and the period between them.
func SendMultipleEvents(numberOfEvents int, period time.Duration) EventsHubOption {
	return compose(
		envOption("MAX_MESSAGES", strconv.Itoa(numberOfEvents)),
		envDuration("PERIOD", period),
	)
}

// --- Options utils

func noop(context.Context, map[string]string) error {
	return nil
}

func compose(options ...EventsHubOption) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		for _, opt := range options {
			if err := opt(ctx, envs); err != nil {
				return err
			}
		}
		return nil
	}
}

func envOptionalOpt(key, value string) EventsHubOption {
	if value != "" {
		return func(ctx context.Context, envs map[string]string) error {
			envs[key] = value
			return nil
		}
	} else {
		return noop
	}
}

func envOption(key, value string) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		envs[key] = value
		return nil
	}
}

func envAdditive(key, value string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		if containedValue, ok := m[key]; ok {
			m[key] = containedValue + "," + value
		} else {
			m[key] = value
		}
		return nil
	}
}

func envDuration(key string, value time.Duration) EventsHubOption {
	return envOption(key, strconv.Itoa(int(math.Ceil(value.Seconds()))))
}
