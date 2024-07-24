/*
Copyright 2021 The Knative Authors

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

package recordevents

import (
	"encoding/json"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	testlib "knative.dev/eventing/test/lib"
)

type EventRecordOption = func(*corev1.Pod, *testlib.Client) error

func compose(options ...EventRecordOption) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		for _, opt := range options {
			if err := opt(pod, client); err != nil {
				return err
			}
		}
		return nil
	}
}

func envOptionalOpt(key, value string) EventRecordOption {
	if value != "" {
		return func(pod *corev1.Pod, client *testlib.Client) error {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: key, Value: value},
			)
			return nil
		}
	} else {
		return noop
	}
}

func envOption(key, value string) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		pod.Spec.Containers[0].Env = append(
			pod.Spec.Containers[0].Env,
			corev1.EnvVar{Name: key, Value: value},
		)
		return nil
	}
}

// EchoEvent is an option to let the recordevents reply with the received event
var EchoEvent EventRecordOption = envOption("REPLY", "true")

// ReplyWithTransformedEvent is an option to let the recordevents reply with the transformed event
func ReplyWithTransformedEvent(replyEventType string, replyEventSource string, replyEventData string) EventRecordOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_EVENT_TYPE", replyEventType),
		envOptionalOpt("REPLY_EVENT_SOURCE", replyEventSource),
		envOptionalOpt("REPLY_EVENT_DATA", replyEventData),
	)
}

// ReplyWithAppendedData is an option to let the recordevents reply with the transformed event with appended data
func ReplyWithAppendedData(appendData string) EventRecordOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_APPEND_DATA", appendData),
	)
}

// InputEvent is an option to provide the event to send when deploying the event sender
func InputEvent(event cloudevents.Event) EventRecordOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(pod *corev1.Pod, client *testlib.Client) error {
			return err
		}
	}
	return envOption("INPUT_EVENT", string(encodedEvent))
}

// AddTracing adds tracing headers when sending events.
func AddTracing() EventRecordOption {
	return envOption("ADD_TRACING", "true")
}

// EnableIncrementalId creates a new incremental id for each sent event.
func EnableIncrementalId() EventRecordOption {
	return envOption("INCREMENTAL_ID", "true")
}

// InputEncoding forces the encoding of the event for each sent event.
func InputEncoding(encoding cloudevents.Encoding) EventRecordOption {
	return envOption("EVENT_ENCODING", encoding.String())
}

// InputHeaders adds the following headers to the sent requests.
func InputHeaders(headers map[string]string) EventRecordOption {
	return envOption("INPUT_HEADERS", serializeHeaders(headers))
}
