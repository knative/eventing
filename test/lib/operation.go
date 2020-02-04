/*
Copyright 2019 The Knative Authors

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

package lib

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"

	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

// LabelNamespace labels the given namespace with the labels map.
func (client *Client) LabelNamespace(labels map[string]string) error {
	namespace := client.Namespace
	nsSpec, err := client.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if nsSpec.Labels == nil {
		nsSpec.Labels = map[string]string{}
	}
	for k, v := range labels {
		nsSpec.Labels[k] = v
	}
	_, err = client.Kube.Kube.CoreV1().Namespaces().Update(nsSpec)
	return err
}

// SendFakeEventToAddressableOrFail will send the given event to the given Addressable.
func (client *Client) SendFakeEventToAddressableOrFail(
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	event *cloudevents.CloudEvent,
) {
	uri, err := client.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		client.T.Fatalf("Failed to get the URI for %v-%s", typemeta, addressableName)
	}
	if err = client.sendFakeEventToAddress(senderName, uri, event); err != nil {
		client.T.Fatalf("Failed to send event %v to %s: %v", event, uri, err)
	}
}

// SendFakeEventWithTracingToAddressableOrFail will send the given event with tracing to the given Addressable.
func (client *Client) SendFakeEventWithTracingToAddressableOrFail(
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	event *cloudevents.CloudEvent,
) {
	uri, err := client.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		client.T.Fatalf("Failed to get the URI for %v-%s", typemeta, addressableName)
	}
	if err = client.sendFakeEventWithTracingToAddress(senderName, uri, event); err != nil {
		client.T.Fatalf("Failed to send event %v with tracing to %s: %v", event, uri, err)
	}
}

// GetAddressableURI returns the URI of the addressable resource.
// To use this function, the given resource must have implemented the Addressable duck-type.
func (client *Client) GetAddressableURI(addressableName string, typeMeta *metav1.TypeMeta) (string, error) {
	namespace := client.Namespace
	metaAddressable := resources.NewMetaResource(addressableName, namespace, typeMeta)
	u, err := duck.GetAddressableURI(client.Dynamic, metaAddressable)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// sendFakeEventToAddress will create a sender pod, which will send the given event to the given url.
func (client *Client) sendFakeEventToAddress(
	senderName string,
	uri string,
	event *cloudevents.CloudEvent,
) error {
	namespace := client.Namespace
	pod, err := resources.EventSenderPod(senderName, uri, event)
	if err != nil {
		return err
	}
	client.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(client.Kube, senderName, namespace); err != nil {
		return err
	}
	return nil
}

// sendFakeEventWithTracingToAddress will create a sender pod, which will send the given event with tracing to the given url.
func (client *Client) sendFakeEventWithTracingToAddress(
	senderName string,
	uri string,
	event *cloudevents.CloudEvent,
) error {
	namespace := client.Namespace
	pod, err := resources.EventSenderTracingPod(senderName, uri, event)
	if err != nil {
		return err
	}
	client.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(client.Kube, senderName, namespace); err != nil {
		return err
	}
	return nil
}

// WaitForResourceReadyOrFail waits for the resource to become ready or fail.
// To use this function, the given resource must have implemented the Status duck-type.
func (client *Client) WaitForResourceReadyOrFail(name string, typemeta *metav1.TypeMeta) {
	namespace := client.Namespace
	metaResource := resources.NewMetaResource(name, namespace, typemeta)
	if err := duck.WaitForResourceReady(client.Dynamic, metaResource); err != nil {
		client.T.Fatalf("Failed to get %v-%s ready: %v", *typemeta, name, err)
	}
}

// WaitForResourcesReadyOrFail waits for resources of the given type in the namespace to become ready or fail.
// To use this function, the given resource must have implemented the Status duck-type.
func (client *Client) WaitForResourcesReadyOrFail(typemeta *metav1.TypeMeta) {
	namespace := client.Namespace
	metaResourceList := resources.NewMetaResourceList(namespace, typemeta)
	if err := duck.WaitForResourcesReady(client.Dynamic, metaResourceList); err != nil {
		client.T.Fatalf("Failed to get all %v resources ready: %v", *typemeta, err)
	}
}

// WaitForAllTestResourcesReady waits until all test resources in the namespace are Ready.
func (client *Client) WaitForAllTestResourcesReady() error {
	// wait for all Knative resources created in this test to become ready.
	if err := client.Tracker.WaitForKResourcesReady(); err != nil {
		return err
	}
	// Explicitly wait for all pods that were created directly by this test to become ready.
	for _, n := range client.podsCreated {
		if err := pkgTest.WaitForPodRunning(client.Kube, n, client.Namespace); err != nil {
			return fmt.Errorf("created Pod %q did not become ready: %v", n, err)
		}
	}
	// FIXME(chizhg): This hacky sleep is added to try mitigating the test flakiness.
	// Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)
	return nil
}

func (client *Client) WaitForAllTestResourcesReadyOrFail() {
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		client.T.Fatalf("Failed to get all test resources ready: %v", err)
	}
}

func (client *Client) WaitForServiceEndpointsOrFail(svcName string, numberOfExpectedEndpoints int) {
	client.T.Logf("Waiting for %d endpoints in service %s", numberOfExpectedEndpoints, svcName)
	if err := pkgTest.WaitForServiceEndpoints(client.Kube, svcName, client.Namespace, numberOfExpectedEndpoints); err != nil {
		client.T.Fatalf("Failed while waiting for %d endpoints in service %s: %v", numberOfExpectedEndpoints, svcName, err)
	}
}
