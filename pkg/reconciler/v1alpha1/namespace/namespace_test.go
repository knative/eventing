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

package namespace

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

const (
	testNS     = "test-namespace"
	brokerName = "default"
)

var (
	falseString = "false"
	trueString  = "true"

	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
)

func init() {
	// Add types to scheme
	v1alpha1.AddToScheme(scheme.Scheme)
}

func TestProvideController(t *testing.T) {
	//TODO(grantr) This needs a mock of manager.Manager. Creating a manager
	// with a fake Config fails because the Manager tries to contact the
	// apiserver.

	// cfg := &rest.Config{
	// 	Host: "http://foo:80",
	// }
	//
	// mgr, err := manager.New(cfg, manager.Options{})
	// if err != nil {
	// 	t.Fatalf("Error creating manager: %v", err)
	// }
	//
	// _, err = ProvideController(mgr)
	// if err != nil {
	// 	t.Fatalf("Error in ProvideController: %v", err)
	// }
}

func TestInjectClient(t *testing.T) {
	r := &reconciler{}
	orig := r.client
	n := fake.NewFakeClient()
	if orig == n {
		t.Errorf("Original and new clients are identical: %v", orig)
	}
	err := r.InjectClient(n)
	if err != nil {
		t.Errorf("Unexpected error injecting the client: %v", err)
	}
	if n != r.client {
		t.Errorf("Unexpected client. Expected: '%v'. Actual: '%v'", n, r.client)
	}
}

func TestInjectConfig(t *testing.T) {
	r := &reconciler{}
	wantCfg := &rest.Config{
		Host: "http://foo",
	}

	err := r.InjectConfig(wantCfg)
	if err != nil {
		t.Fatalf("Unexpected error injecting the config: %v", err)
	}

	gotCfg := r.restConfig
	if diff := cmp.Diff(wantCfg, gotCfg); diff != "" {
		t.Errorf("Unexpected config (-want, +got): %v", diff)
	}

	wantDynClient, err := dynamic.NewForConfig(wantCfg)
	if err != nil {
		t.Fatalf("Unexpected error generating dynamic client: %v", err)
	}

	// Since dynamicClient doesn't export any fields, we can only test its type.
	switch r.dynamicClient.(type) {
	case dynamic.Interface:
		// ok
	default:
		t.Errorf("Unexpected dynamicClient type. Expected: %T, Got: %T", wantDynClient, r.dynamicClient)
	}
}

func TestNamespaceMapper_Map(t *testing.T) {
	m := &namespaceMapper{}

	req := handler.MapObject{
		Meta:   makeBroker().GetObjectMeta(),
		Object: makeBroker(),
	}
	actual := m.Map(req)
	expected := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: "",
				Name:      testNS,
			},
		},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Unexpected reconcile requests (-want +got): %v", diff)
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "Namespace not found",
		},
		{
			Name:   "Namespace.Get fails",
			Scheme: scheme.Scheme,
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*corev1.Namespace); ok {
							return controllertesting.Handled, errors.New("test error getting the NS")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the NS",
		},
		{
			Name:   "Namespace is not annotated",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(nil),
			},
			WantAbsent: []runtime.Object{
				makeBroker(),
			},
		},
		{
			Name:   "Namespace is annotated off",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&falseString),
			},
			WantAbsent: []runtime.Object{
				makeBroker(),
			},
		},
		{
			Name:   "Namespace is being deleted",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeDeletingNamespace(),
			},
			WantAbsent: []runtime.Object{
				makeBroker(),
			},
		},
		{
			Name:   "Broker.Get fails",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&trueString),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error getting the Broker")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the Broker",
			WantAbsent: []runtime.Object{
				makeBroker(),
			},
		},
		{
			Name:   "Broker Found",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&trueString),
				makeBroker(),
			},
		},
		{
			Name:   "Broker.Create fails",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&trueString),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error creating the Broker")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating the Broker",
		},
		{
			Name:   "Broker created",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&trueString),
			},
			WantPresent: []runtime.Object{
				makeBroker(),
			},
			WantEvent: []corev1.Event{
				{
					Reason: brokerCreated, Type: corev1.EventTypeNormal,
				},
			},
		},
	}
	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()
		recorder := tc.GetEventRecorder()

		r := &reconciler{
			client:        c,
			dynamicClient: dc,
			restConfig:    &rest.Config{},
			recorder:      recorder,
			logger:        zap.NewNop(),
		}
		tc.ReconcileKey = fmt.Sprintf("%s/%s", "", testNS)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeNamespace(annotationValue *string) *corev1.Namespace {
	annotations := map[string]string{}
	if annotationValue != nil {
		annotations["eventing.knative.dev/inject"] = *annotationValue
	}

	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        testNS,
			Annotations: annotations,
		},
	}
}

func makeDeletingNamespace() *corev1.Namespace {
	ns := makeNamespace(&trueString)
	ns.DeletionTimestamp = &deletionTime
	return ns
}

func makeBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      brokerName,
			Labels: map[string]string{
				"eventing.knative.dev/namespaceInjected": "true",
			},
		},
	}
}
