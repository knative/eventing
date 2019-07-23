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

package common

import (
	"time"

	"github.com/knative/eventing/test/base"
	"github.com/knative/eventing/test/base/resources"
	pkgTest "github.com/knative/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SendFakeEventToAddressable will send the given event to the given Addressable.
func (client *Client) SendFakeEventToAddressable(
	senderName,
	addressableName string,
	typemeta *metav1.TypeMeta,
	event *resources.CloudEvent,
) error {
	uri, err := client.GetAddressableURI(addressableName, typemeta)
	if err != nil {
		return err
	}
	return client.sendFakeEventToAddress(senderName, uri, event)
}

// GetAddressableURI returns the URI of the addressable resource.
// To use this function, the given resource must have implemented the Addressable duck-type.
func (client *Client) GetAddressableURI(addressableName string, typemeta *metav1.TypeMeta) (string, error) {
	namespace := client.Namespace
	metaAddressable := resources.NewMetaResource(addressableName, namespace, typemeta)
	return base.GetAddressableURI(client.Dynamic, metaAddressable)
}

// sendFakeEventToAddress will create a sender pod, which will send the given event to the given url.
func (client *Client) sendFakeEventToAddress(
	senderName string,
	uri string,
	event *resources.CloudEvent,
) error {
	namespace := client.Namespace
	pod := resources.EventSenderPod(senderName, uri, event)
	client.CreatePodOrFail(pod)
	if err := pkgTest.WaitForPodRunning(client.Kube, senderName, namespace); err != nil {
		return err
	}
	return nil
}

// WaitForResourceReady waits for the resource to become ready.
// To use this function, the given resource must have implemented the Status duck-type.
func (client *Client) WaitForResourceReady(name string, typemeta *metav1.TypeMeta) error {
	namespace := client.Namespace
	metaResource := resources.NewMetaResource(name, namespace, typemeta)
	if err := base.WaitForResourceReady(client.Dynamic, metaResource); err != nil {
		return err
	}
	return nil
}

// WaitForResourcesReady waits for resources of the given type in the namespace to become ready.
// To use this function, the given resource must have implemented the Status duck-type.
func (client *Client) WaitForResourcesReady(typemeta *metav1.TypeMeta) error {
	namespace := client.Namespace
	metaResourceList := resources.NewMetaResourceList(namespace, typemeta)
	if err := base.WaitForResourcesReady(client.Dynamic, metaResourceList); err != nil {
		return err
	}
	return nil
}

// WaitForAllTestResourcesReady waits until all test resources in the namespace are Ready.
func (client *Client) WaitForAllTestResourcesReady() error {
	// wait for all Knative resources created in this test to become ready.
	client.Tracker.WaitForKResourcesReady()
	// explicitly wait for all pods to become ready.
	if err := pkgTest.WaitForAllPodsRunning(client.Kube, client.Namespace); err != nil {
		return err
	}
	// FIXME(Fredy-Z): This hacky sleep is added to try mitigating the test flakiness.
	// Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)
	return nil
}
