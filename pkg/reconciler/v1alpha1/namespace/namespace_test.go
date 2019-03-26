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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testNS     = "test-namespace"
	brokerName = "default"
)

var (
	disabled = "disabled"
	enabled  = "enabled"

	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		brokerCreated:             {Reason: brokerCreated, Type: corev1.EventTypeNormal},
		serviceAccountCreated:     {Reason: serviceAccountCreated, Type: corev1.EventTypeNormal},
		serviceAccountRBACCreated: {Reason: serviceAccountRBACCreated, Type: corev1.EventTypeNormal},
	}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestProvideController(t *testing.T) {
	// TODO(grantr) This needs a mock of manager.Manager. Creating a manager
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

func TestNamespaceMapper_Map(t *testing.T) {
	m := &namespaceMapper{
		name: makeBroker().Name,
	}

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
			Name:   "Namespace is not labeled",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(nil),
			},
			WantAbsent: []runtime.Object{
				makeBroker(),
			},
		},
		{
			Name:   "Namespace is labeled disabled",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&disabled),
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
				makeNamespace(&enabled),
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
			WantEvent: []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		},
		{
			Name:   "Broker Found",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&enabled),
				makeBroker(),
			},
			WantEvent: []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		},
		{
			Name:   "Broker.Create fails",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&enabled),
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
			WantEvent:  []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		},
		{
			Name:   "Broker created",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeNamespace(&enabled),
			},
			WantPresent: []runtime.Object{
				makeBroker(),
			},
			WantEvent: []corev1.Event{
				events[serviceAccountCreated],
				events[serviceAccountRBACCreated],
				events[brokerCreated]},
		},
	}
	for _, tc := range testCases {
		c := tc.GetClient()
		recorder := tc.GetEventRecorder()

		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),
		}
		tc.ReconcileKey = fmt.Sprintf("%s/%s", "", testNS)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeNamespace(labelValue *string) *corev1.Namespace {
	labels := map[string]string{}
	if labelValue != nil {
		labels["knative-eventing-injection"] = *labelValue
	}

	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNS,
			Labels: labels,
		},
	}
}

func makeDeletingNamespace() *corev1.Namespace {
	ns := makeNamespace(&enabled)
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
