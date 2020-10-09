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

package sender

import (
	"encoding/json"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"
)

// EnableTracing enables tracing in sender pod
func EnableTracing() func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-add-tracing",
			"true",
		)
	}
}

// EnableIncrementalId creates a new incremental id for each event sent from the sender pod. Supported only by event-sender
func EnableIncrementalId() func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-incremental-id",
			"true",
		)
	}
}

// WithEncoding forces the encoding of the event to send from the sender pod. Supported only by event-sender
func WithEncoding(encoding cloudevents.Encoding) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-event-encoding",
			encoding.String(),
		)
	}
}

// WithResponseSink sends the response information as CloudEvent to another sink
func WithResponseSink(responseSink string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-response-sink",
			responseSink,
		)
	}
}

// WithEncoding forces the encoding of the event to send from the sender pod. Supported only by event-sender
func WithAdditionalHeaders(headers map[string]string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-additional-headers",
			serializeHeaders(headers),
		)
	}
}

// EventSenderPod creates a Pod that sends events to the given address.
func EventSenderPod(imageName string, name string, sink string, event cloudevents.Event, options ...func(*corev1.Pod)) (*corev1.Pod, error) {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-sink",
		sink,
		"-event",
		string(encodedEvent),
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            args,
			}},
			// Never restart the event sender Pod.
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	for _, opt := range options {
		opt(p)
	}

	return p, nil
}

// WithMethod configures the method used to send the http request. Supported only by request-sender
func WithMethod(method string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-method",
			method,
		)
	}
}

// EventSenderPod creates a Pod that sends http requests to the given address.
func RequestSenderPod(imageName string, name string, sink string, headers map[string]string, body string, options ...func(*corev1.Pod)) (*corev1.Pod, error) {
	args := []string{
		"-sink",
		sink,
		"-headers",
		serializeHeaders(headers),
		"-body",
		body,
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            args,
			}},
			// Never restart the event sender Pod.
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	for _, opt := range options {
		opt(p)
	}

	return p, nil
}

func serializeHeaders(headers map[string]string) string {
	kv := make([]string, 0, len(headers))
	for k, v := range headers {
		kv = append(kv, k+"="+v)
	}
	return strings.Join(kv, ",")
}
