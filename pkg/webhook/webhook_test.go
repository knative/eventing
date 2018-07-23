/*
Copyright 2018 The Knative Authors
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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/knative/eventing/pkg"

	"github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/mattbaird/jsonpatch"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func newDefaultOptions() ControllerOptions {
	return ControllerOptions{
		ServiceName:      "eventing-webhook",
		ServiceNamespace: pkg.GetEventingSystemNamespace(),
		Port:             443,
		SecretName:       "eventing-webhook-certs",
		WebhookName:      "webhook.eventing.knative.dev",
	}
}

const (
	testNamespace        = "test-namespace"
	testBusName          = "test-bus"
	testClusterBusName   = "test-clusterbus"
	testChannelName      = "test-channel"
	testSubscriptionName = "test-subscription"
)

var (
	testCtx = context.TODO()
)

func newRunningTestAdmissionController(t *testing.T, options ControllerOptions) (
	kubeClient *fakekubeclientset.Clientset,
	ac *AdmissionController,
	stopCh chan struct{}) {
	// Create fake clients
	kubeClient = fakekubeclientset.NewSimpleClientset()

	ac, err := NewAdmissionController(kubeClient, options)
	if err != nil {
		t.Fatalf("Failed to create new admission controller: %s", err)
	}
	stopCh = make(chan struct{})
	go func() {
		if err := ac.Run(stopCh); err != nil {
			t.Fatalf("Error running controller: %v", err)
		}
	}()
	ac.Run(stopCh)
	return
}

func newNonRunningTestAdmissionController(t *testing.T, options ControllerOptions) (
	kubeClient *fakekubeclientset.Clientset,
	ac *AdmissionController) {
	// Create fake clients
	kubeClient = fakekubeclientset.NewSimpleClientset()

	ac, err := NewAdmissionController(kubeClient, options)
	if err != nil {
		t.Fatalf("Failed to create new admission controller: %s", err)
	}
	return
}

func TestDeleteAllowed(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())

	req := admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Delete,
	}

	resp := ac.admit(testCtx, &req)
	if !resp.Allowed {
		t.Fatalf("unexpected denial of delete")
	}
}

func TestConnectAllowed(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())

	req := admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Connect,
	}

	resp := ac.admit(testCtx, &req)
	if !resp.Allowed {
		t.Fatalf("unexpected denial of connect")
	}
}

func TestUnknownKindFails(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())

	req := admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Garbage"},
	}

	expectFailsWith(t, ac.admit(testCtx, &req), "unhandled kind")
}

func TestValidBusParameterNamePasses(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Bus"},
	}
	validName := "ok_param.name"
	bus := createBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Subscription = &[]v1alpha1.Parameter{{Name: validName}}
	marshaled, err := json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectAllowed(t, ac.admit(testCtx, req))

	validName = "simple-name"
	bus = createBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Channel = &[]v1alpha1.Parameter{{Name: validName}}
	marshaled, err = json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectAllowed(t, ac.admit(testCtx, req))
}

func TestInvalidBusParameterNameFails(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Bus"},
	}
	invalidName := "paramètre"
	bus := createBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Subscription = &[]v1alpha1.Parameter{{Name: invalidName}}
	marshaled, err := json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "invalid parameter name Spec.Parameters.Subscription.paramètre")

	invalidName = "param/name"
	bus = createBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Channel = &[]v1alpha1.Parameter{{Name: invalidName}}
	marshaled, err = json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "invalid parameter name Spec.Parameters.Channel.param/name")
}

func TestInvalidClusterBusParameterNameFails(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "ClusterBus"},
	}
	invalidName := "paramètre"
	bus := createClusterBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Subscription = &[]v1alpha1.Parameter{{Name: invalidName}}
	marshaled, err := json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "invalid parameter name Spec.Parameters.Subscription.paramètre")

	invalidName = "param/name"
	bus = createClusterBus(testBusName, "foobar/dispatcher")
	bus.Spec.Parameters.Channel = &[]v1alpha1.Parameter{{Name: invalidName}}
	marshaled, err = json.Marshal(bus)
	if err != nil {
		t.Fatalf("Failed to marshal bus: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "invalid parameter name Spec.Parameters.Channel.param/name")
}

func TestInvalidNewChannelNameFails(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Channel"},
	}
	invalidName := "channel.example"
	channel := createChannel(invalidName, testBusName, "")
	marshaled, err := json.Marshal(channel)
	if err != nil {
		t.Fatalf("Failed to marshal channel: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "Invalid resource name")

	invalidName = strings.Repeat("a", 64)
	channel = createChannel(invalidName, testBusName, "")
	marshaled, err = json.Marshal(channel)
	if err != nil {
		t.Fatalf("Failed to marshal channel: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "Invalid resource name")
}

func TestValidNewChannelObject(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	resp := ac.admit(testCtx, createValidCreateChannel())
	expectAllowed(t, resp)
	expectPatches(t, resp.Patch, []jsonpatch.JsonPatchOperation{})
}

func TestValidChannelNSBusNoChanges(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	old := createChannel(testChannelName, testBusName, "")
	new := createChannel(testChannelName, testBusName, "")
	resp := ac.admit(testCtx, createUpdateChannel(&old, &new))
	expectAllowed(t, resp)
	expectPatches(t, resp.Patch, []jsonpatch.JsonPatchOperation{})
}

func TestValidChannelClusterBusNoChanges(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	old := createChannel(testChannelName, "", testClusterBusName)
	new := createChannel(testChannelName, "", testClusterBusName)
	resp := ac.admit(testCtx, createUpdateChannel(&old, &new))
	expectAllowed(t, resp)
	expectPatches(t, resp.Patch, []jsonpatch.JsonPatchOperation{})
}

func TestInvalidNewSubscriptionNameFails(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Subscription"},
	}
	invalidName := "subscription.example"
	subscription := createSubscription(invalidName, "channel-name")
	marshaled, err := json.Marshal(subscription)
	if err != nil {
		t.Fatalf("Failed to marshal subscription: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "Invalid resource name")

	invalidName = strings.Repeat("a", 64)
	subscription = createSubscription(invalidName, "channel-name")
	marshaled, err = json.Marshal(subscription)
	if err != nil {
		t.Fatalf("Failed to marshal subscription: %s", err)
	}
	req.Object.Raw = marshaled
	expectFailsWith(t, ac.admit(testCtx, req), "Invalid resource name")
}

func TestValidNewSubscriptionObject(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	resp := ac.admit(testCtx, createValidCreateSubscription())
	expectAllowed(t, resp)
	expectPatches(t, resp.Patch, []jsonpatch.JsonPatchOperation{})
}

func TestValidSubscriptionNoChanges(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	old := createSubscription(testSubscriptionName, testChannelName)
	new := createSubscription(testSubscriptionName, testChannelName)
	resp := ac.admit(testCtx, createUpdateSubscription(&old, &new))
	expectAllowed(t, resp)
	expectPatches(t, resp.Patch, []jsonpatch.JsonPatchOperation{})
}

func TestValidWebhook(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	createDeployment(ac)
	ac.register(testCtx, ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations(), []byte{})
	_, err := ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(ac.options.WebhookName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to create webhook: %s", err)
	}
}

func TestUpdatingWebhook(t *testing.T) {
	_, ac := newNonRunningTestAdmissionController(t, newDefaultOptions())
	webhook := &admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ac.options.WebhookName,
		},
		Webhooks: []admissionregistrationv1beta1.Webhook{{
			Name:         ac.options.WebhookName,
			Rules:        []admissionregistrationv1beta1.RuleWithOperations{{}},
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{},
		}},
	}

	createDeployment(ac)
	createWebhook(ac, webhook)
	ac.register(testCtx, ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations(), []byte{})
	currentWebhook, _ := ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(ac.options.WebhookName, metav1.GetOptions{})
	if reflect.DeepEqual(currentWebhook.Webhooks, webhook.Webhooks) {
		t.Fatalf("Expected webhook to be updated")
	}
}

func createDeployment(ac *AdmissionController) {
	deployment := &extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventingWebhookDeployment,
			Namespace: pkg.GetEventingSystemNamespace(),
		},
	}
	ac.client.ExtensionsV1beta1().Deployments(pkg.GetEventingSystemNamespace()).Create(deployment)
}

func createWebhook(ac *AdmissionController, webhook *admissionregistrationv1beta1.MutatingWebhookConfiguration) {
	client := ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
	_, err := client.Create(webhook)
	if err != nil {
		panic(fmt.Sprintf("failed to create test webhook: %s", err))
	}
}

func expectAllowed(t *testing.T, resp *admissionv1beta1.AdmissionResponse) {
	if !resp.Allowed {
		t.Errorf("Expected allowed, but failed with %+v", resp.Result)
	}
}

func expectFailsWith(t *testing.T, resp *admissionv1beta1.AdmissionResponse, contains string) {
	if resp.Allowed {
		t.Errorf("expected denial, got allowed")
		return
	}
	if !strings.Contains(resp.Result.Message, contains) {
		t.Errorf("expected failure containing %q got %q", contains, resp.Result.Message)
	}
}

func expectPatches(t *testing.T, a []byte, e []jsonpatch.JsonPatchOperation) {
	var actual []jsonpatch.JsonPatchOperation
	// Keep track of the patches we've found
	foundExpected := make([]bool, len(e))
	foundActual := make([]bool, len(e))

	err := json.Unmarshal(a, &actual)
	if err != nil {
		t.Errorf("failed to unmarshal patches: %s", err)
		return
	}
	if len(actual) != len(e) {
		t.Errorf("unexpected number of patches %d expected %d\n%+v\n%+v", len(actual), len(e), actual, e)
	}
	// Make sure all the expected patches are found
	for i, expectedPatch := range e {
		for j, actualPatch := range actual {
			if actualPatch.Json() == expectedPatch.Json() {
				foundExpected[i] = true
				foundActual[j] = true
			}
		}
	}
	for i, f := range foundExpected {
		if !f {
			t.Errorf("did not find %+v in actual patches: %q", e[i], actual)
		}
	}
	for i, f := range foundActual {
		if !f {
			t.Errorf("Extra patch found %+v in expected patches: %q", a[i], e)
		}
	}
}

func createBus(busName string, dispatcherImage string) v1alpha1.Bus {
	return v1alpha1.Bus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      busName,
		},
		Spec: v1alpha1.BusSpec{
			Dispatcher: v1.Container{
				Image: dispatcherImage,
			},
			Parameters: &v1alpha1.BusParameters{
				Channel:      &[]v1alpha1.Parameter{},
				Subscription: &[]v1alpha1.Parameter{},
			},
		},
	}
}

func createClusterBus(busName string, dispatcherImage string) v1alpha1.ClusterBus {
	return v1alpha1.ClusterBus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      busName,
		},
		Spec: v1alpha1.BusSpec{
			Dispatcher: v1.Container{
				Image: dispatcherImage,
			},
			Parameters: &v1alpha1.BusParameters{
				Channel:      &[]v1alpha1.Parameter{},
				Subscription: &[]v1alpha1.Parameter{},
			},
		},
	}
}

func createBaseUpdateChannel() *admissionv1beta1.AdmissionRequest {
	return &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Update,
		Kind:      metav1.GroupVersionKind{Kind: "Channel"},
	}
}

func createUpdateChannel(old, new *v1alpha1.Channel) *admissionv1beta1.AdmissionRequest {
	req := createBaseUpdateChannel()
	marshaled, err := json.Marshal(old)
	if err != nil {
		panic("failed to marshal channel")
	}
	req.Object.Raw = marshaled
	marshaledOld, err := json.Marshal(new)
	if err != nil {
		panic("failed to marshal channel")
	}
	req.OldObject.Raw = marshaledOld
	return req
}

func createCreateChannel(channel v1alpha1.Channel) *admissionv1beta1.AdmissionRequest {
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Channel"},
	}
	marshaled, err := json.Marshal(channel)
	if err != nil {
		panic("failed to marshal channel")
	}
	req.Object.Raw = marshaled
	return req
}

func createValidCreateChannel() *admissionv1beta1.AdmissionRequest {
	return createCreateChannel(createChannel(testChannelName, testBusName, ""))
}

func createChannel(channelName string, busName, clusterBusName string) v1alpha1.Channel {
	return v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      channelName,
		},
		Spec: v1alpha1.ChannelSpec{
			Bus:        busName,
			ClusterBus: clusterBusName,
		},
	}
}

func createBaseUpdateSubscription() *admissionv1beta1.AdmissionRequest {
	return &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Update,
		Kind:      metav1.GroupVersionKind{Kind: "Subscription"},
	}
}

func createUpdateSubscription(old, new *v1alpha1.Subscription) *admissionv1beta1.AdmissionRequest {
	req := createBaseUpdateSubscription()
	marshaled, err := json.Marshal(old)
	if err != nil {
		panic("failed to marshal subscription")
	}
	req.Object.Raw = marshaled
	marshaledOld, err := json.Marshal(new)
	if err != nil {
		panic("failed to marshal subscription")
	}
	req.OldObject.Raw = marshaledOld
	return req
}

func createCreateSubscription(subscription v1alpha1.Subscription) *admissionv1beta1.AdmissionRequest {
	req := &admissionv1beta1.AdmissionRequest{
		Operation: admissionv1beta1.Create,
		Kind:      metav1.GroupVersionKind{Kind: "Subscription"},
	}
	marshaled, err := json.Marshal(subscription)
	if err != nil {
		panic("failed to marshal subscription")
	}
	req.Object.Raw = marshaled
	return req
}

func createValidCreateSubscription() *admissionv1beta1.AdmissionRequest {
	return createCreateSubscription(createSubscription(testSubscriptionName, testChannelName))
}

func createSubscription(subscriptionName string, channelName string) v1alpha1.Subscription {
	return v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      subscriptionName,
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: channelName,
		},
	}
}
