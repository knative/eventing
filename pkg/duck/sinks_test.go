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

package duck

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/knative/eventing/pkg/reconciler"
)

var (
	addressableDNS = "addressable.sink.svc.cluster.local"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"

	unaddressableName       = "testunaddressable"
	unaddressableKind       = "KResource"
	unaddressableAPIVersion = "duck.knative.dev/v1alpha1"
	unaddressableResource   = "kresources.duck.knative.dev"

	testNS = "testnamespace"
)

func init() {
	// Add types to scheme
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestGetSinkURI(t *testing.T) {
	testCases := map[string]struct {
		objects   []runtime.Object
		namespace string
		want      string
		wantErr   error
		ref       *corev1.ObjectReference
	}{
		"happy": {
			objects: []runtime.Object{
				getAddressable(),
			},
			namespace: testNS,
			ref:       getAddressableRef(),
			want:      fmt.Sprintf("http://%s", addressableDNS),
		},
		"nil hostname": {
			objects: []runtime.Object{
				getAddressableNilHostname(),
			},
			namespace: testNS,
			ref:       getUnaddressableRef(),
			wantErr:   fmt.Errorf(`sink &ObjectReference{Kind:%s,Namespace:%s,Name:%s,UID:,APIVersion:%s,ResourceVersion:,FieldPath:,} contains an empty hostname`, unaddressableKind, testNS, unaddressableName, unaddressableAPIVersion),
		},
		"nil sink": {
			objects: []runtime.Object{
				getAddressableNilHostname(),
			},
			namespace: testNS,
			ref:       nil,
			wantErr:   fmt.Errorf(`sink ref is nil`),
		},
		"nil address": {
			objects: []runtime.Object{
				getAddressableNilAddress(),
			},
			namespace: testNS,
			ref:       nil,
			wantErr:   fmt.Errorf(`sink ref is nil`),
		},
		"notSink": {
			objects: []runtime.Object{
				getAddressableNoStatus(),
			},
			namespace: testNS,
			ref:       getUnaddressableRef(),
			wantErr:   fmt.Errorf(`sink &ObjectReference{Kind:%s,Namespace:%s,Name:%s,UID:,APIVersion:%s,ResourceVersion:,FieldPath:,} does not contain address`, unaddressableKind, testNS, unaddressableName, unaddressableAPIVersion),
		},
		"notFound": {
			namespace: testNS,
			ref:       getUnaddressableRef(),
			wantErr:   fmt.Errorf(`Error fetching sink &ObjectReference{Kind:%s,Namespace:%s,Name:%s,UID:,APIVersion:%s,ResourceVersion:,FieldPath:,} for source "%s": %s "%s" not found`, unaddressableKind, testNS, unaddressableName, unaddressableAPIVersion, unaddressableName, unaddressableResource, unaddressableName),
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			sr := NewSinkReconciler(
				reconciler.Options{
					DynamicClientSet: fake.NewSimpleDynamicClient(scheme.Scheme, tc.objects...),
				},
				func(string) {},
			)
			sourceName := "nilRef"
			if tc.ref != nil {
				sourceName = tc.ref.Name
			}
			uri, gotErr := sr.GetSinkURI(tc.ref, getAddressable(), sourceName)
			if gotErr != nil {
				if tc.wantErr != nil {
					if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
						t.Errorf("%s: unexpected error (-want, +got) = %v", n, diff)
					}
				} else {
					t.Errorf("%s: unexpected error %v", n, gotErr.Error())
				}
			}
			if gotErr == nil {
				got := uri
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("%s: unexpected object (-want, +got) = %v", n, diff)
				}
			}
		})
	}
}

func getAddressable() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       addressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": addressableDNS,
				},
			},
		},
	}
}

func getAddressableNoStatus() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": unaddressableAPIVersion,
			"kind":       unaddressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      unaddressableName,
			},
		},
	}
}

func getAddressableNilAddress() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": unaddressableAPIVersion,
			"kind":       unaddressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      unaddressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}(nil),
			},
		},
	}
}

func getAddressableNilHostname() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": unaddressableAPIVersion,
			"kind":       unaddressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      unaddressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": nil,
				},
			},
		},
	}
}

func getAddressableRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       addressableKind,
		Name:       addressableName,
		APIVersion: addressableAPIVersion,
		Namespace:  testNS,
	}
}

func getUnaddressableRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       unaddressableKind,
		Name:       unaddressableName,
		APIVersion: unaddressableAPIVersion,
		Namespace:  testNS,
	}
}
