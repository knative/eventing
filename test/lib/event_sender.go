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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/test/lib/resources/sender"
)

// SendEventToAddressable will send the given event to the given Addressable.
func (c *Client) SendEventToAddressable(
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	event cloudevents.Event,
	option ...sender.EventSenderOption,
) {
	uri, err := c.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		c.T.Fatalf("Failed to get the URI for %v-%s", typemeta, addressableName)
	}
	if err = c.SendEvent(senderName, uri, event, option...); err != nil {
		c.T.Fatalf("Failed to send event %v with tracing to %s: %v", event, uri, err)
	}
}

// SendEvent will create a sender pod, which will send the given event to the given url.
func (c *Client) SendEvent(
	senderName string,
	uri string,
	event cloudevents.Event,
	option ...sender.EventSenderOption,
) error {
	namespace := c.Namespace
	pod, err := sender.EventSenderPod("event-sender", senderName, uri, event, option...)
	if err != nil {
		return err
	}
	c.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(c.Kube, senderName, namespace); err != nil {
		return err
	}
	return nil
}
