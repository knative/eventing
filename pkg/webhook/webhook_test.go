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

	"github.com/knative/eventing/pkg/system"

	"github.com/mattbaird/jsonpatch"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "test-namespace"
)

func newDefaultOptions() ControllerOptions {
	return ControllerOptions{
		ServiceName:      "eventing-webhook",
		ServiceNamespace: system.Namespace,
		Port:             443,
		SecretName:       "eventing-webhook-certs",
		WebhookName:      "webhook.eventing.knative.dev",
	}
}

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
			Namespace: system.Namespace,
		},
	}
	ac.client.ExtensionsV1beta1().Deployments(system.Namespace).Create(deployment)
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
