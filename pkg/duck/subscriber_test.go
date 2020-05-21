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

package duck

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"k8s.io/client-go/kubernetes/scheme"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

var (
	testNS = "testnamespace"
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
	actual, err := ResourceInterface(&badDynamicInterface{}, testNS, schema.GroupVersionKind{})
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
