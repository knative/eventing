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

package lib

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/test/lib/sender"
)

// SendEventToAddressable will send the given event to the given Addressable.
func (c *Client) SendEventToAddressable(
	ctx context.Context,
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	event cloudevents.Event,
	option ...func(*corev1.Pod),
) {
	uri, err := c.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		c.T.Fatalf("Failed to get the URI for %+v-%s", typemeta, addressableName)
	}
	c.SendEvent(ctx, senderName, uri, event, option...)
}

// SendEvent will create a sender pod, which will send the given event to the given url.
func (c *Client) SendEvent(
	ctx context.Context,
	senderName string,
	uri string,
	event cloudevents.Event,
	option ...func(*corev1.Pod),
) {
	namespace := c.Namespace
	pod, err := sender.EventSenderPod("event-sender", senderName, uri, event, option...)
	if err != nil {
		c.T.Fatalf("Failed to create event-sender pod: %+v", errors.WithStack(err))
	}
	c.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(ctx, c.Kube, senderName, namespace); err != nil {
		c.T.Fatalf("Failed to send event %v to %s: %+v", event, uri, errors.WithStack(err))
	}
}

// SendRequestToAddressable will send the given request to the given Addressable.
func (c *Client) SendRequestToAddressable(
	ctx context.Context,
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	headers map[string]string,
	body string,
	option ...func(*corev1.Pod),
) {
	uri, err := c.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		c.T.Fatalf("Failed to get the URI for %+v-%s", typemeta, addressableName)
	}
	c.SendRequest(ctx, senderName, uri, headers, body, option...)
}

// SendRequest will create a sender pod, which will send the given request to the given url.
func (c *Client) SendRequest(
	ctx context.Context,
	senderName string,
	uri string,
	headers map[string]string,
	body string,
	option ...func(*corev1.Pod),
) {
	namespace := c.Namespace
	pod, err := sender.RequestSenderPod("request-sender", senderName, uri, headers, body, option...)
	if err != nil {
		c.T.Fatalf("Failed to create request-sender pod: %+v", errors.WithStack(err))
	}
	c.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(ctx, c.Kube, senderName, namespace); err != nil {
		c.T.Fatalf("Failed to send request to %s: %+v", uri, errors.WithStack(err))
	}
}
