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

package resolve

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	testNS = "test-namespace"
)

var (
	dnsName = "dns-name"

	channelAddress = "test-channel.hostname"
	channelURL     = fmt.Sprintf("http://%s/", channelAddress)

	legacyCallableAddress = "legacy-callable.domain-internal"
	legacyCallableURL     = fmt.Sprintf("http://%s/", legacyCallableAddress)
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestDomainToURL(t *testing.T) {
	d := "default-broker.default.svc.cluster.local"
	e := fmt.Sprintf("http://%s/", d)
	if actual := DomainToURL(d); e != actual {
		t.Fatalf("Unexpected domain. Expected '%v', actually '%v'", e, actual)
	}
}

func TestResourceInterface_BadDynamicInterface(t *testing.T) {
	actual, err := ResourceInterface(&badDynamicInterface{}, testNS, &corev1.ObjectReference{})
	if err.Error() != "failed to create dynamic client resource" {
		t.Fatalf("Unexpected error '%v'", err)
	}
	if actual != nil {
		t.Fatalf("Unexpected actual. Expected nil. Actual '%v'", actual)
	}
}

type badDynamicInterface struct{}

var _ dynamic.Interface = &badDynamicInterface{}

func (badDynamicInterface) Resource(_ schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return nil
}

func TestObjectReference_BadDynamicInterface(t *testing.T) {
	actual, err := ObjectReference(context.TODO(), &badDynamicInterface{}, testNS, &corev1.ObjectReference{})
	if err.Error() != "failed to create dynamic client resource" {
		t.Fatalf("Unexpected error '%v'", err)
	}
	if actual != nil {
		t.Fatalf("Unexpected actual. Expected nil. Actual '%v'", actual)
	}
}

func TestSubscriberSpec(t *testing.T) {
	testCases := map[string]struct {
		Sub         *v1alpha1.SubscriberSpec
		Objects     []runtime.Object
		Expected    string
		ExpectedErr string
	}{
		"nil": {
			Sub:      nil,
			Expected: "",
		},
		"empty": {
			Sub:      &v1alpha1.SubscriberSpec{},
			Expected: "",
		},
		"DNS Name": {
			Sub: &v1alpha1.SubscriberSpec{
				DNSName: &dnsName,
			},
			Expected: dnsName,
		},
		"Doesn't exist": {
			Sub: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "doesnt-exist",
				},
			},
			ExpectedErr: "services \"doesnt-exist\" not found",
		},
		"K8s Service": {
			Sub: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "does-exist",
				},
			},
			Objects: []runtime.Object{
				k8sService("does-exist"),
			},
			Expected: fmt.Sprintf("http://does-exist.%s.svc.cluster.local/", testNS),
		},
		"Addressable": {
			Sub: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "eventing.knative.dev/v1alpha1",
					Kind:       "Channel",
					Name:       "does-exist",
				},
			},
			Objects: []runtime.Object{
				channel("does-exist"),
			},
			Expected: channelURL,
		},
		"Legacy Callable": {
			Sub: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "eventing.knative.dev/v1alpha1",
					Kind:       "LegacyCallable",
					Name:       "does-exist",
				},
			},
			Objects: []runtime.Object{
				legacyCallable("does-exist"),
			},
			Expected: legacyCallableURL,
		},
		"Non-Addressable": {
			Sub: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "does-exist",
				},
			},
			Objects: []runtime.Object{
				configMap("does-exist"),
			},
			ExpectedErr: "status does not contain address",
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			dc := fake.NewSimpleDynamicClient(scheme.Scheme, tc.Objects...)

			actual, err := SubscriberSpec(context.TODO(), dc, testNS, tc.Sub)
			if err != nil {
				if tc.ExpectedErr == "" || tc.ExpectedErr != err.Error() {
					t.Fatalf("Unexpected error. Expected '%s'. Actual '%s'.", tc.ExpectedErr, err.Error())
				}
			}
			if tc.Expected != actual {
				t.Fatalf("Unexpected URL. Expected '%s'. Actual '%s'", tc.Expected, actual)
			}
		})
	}
}

func k8sService(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      name,
			},
		},
	}
}

func channel(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "eventing.knative.dev/v1alpha1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      name,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": channelAddress,
				},
			},
		},
	}
}

func legacyCallable(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "eventing.knative.dev/v1alpha1",
			"kind":       "LegacyCallable",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      name,
			},
			"status": map[string]interface{}{
				"domainInternal": legacyCallableAddress,
			},
		},
	}
}

func configMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      name,
			},
		},
	}
}
