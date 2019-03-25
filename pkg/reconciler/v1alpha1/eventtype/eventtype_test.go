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

package eventtype

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNS          = "test-namespace"
	eventTypeName   = "test-eventtype"
	eventTypeType   = "test-type"
	eventTypeBroker = "test-broker"
)

var (
	trueVal = true
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// Map of events to set test cases' expectations easier.
	events = map[string]corev1.Event{
		eventTypeReconciled:         {Reason: eventTypeReconciled, Type: corev1.EventTypeNormal},
		eventTypeReconcileFailed:    {Reason: eventTypeReconcileFailed, Type: corev1.EventTypeWarning},
		eventTypeUpdateStatusFailed: {Reason: eventTypeUpdateStatusFailed, Type: corev1.EventTypeWarning},
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

func TestInjectConfig(t *testing.T) {
	r := &reconciler{}
	wantCfg := &rest.Config{
		Host: "http://foo",
	}

	err := r.InjectConfig(wantCfg)
	if err != nil {
		t.Fatalf("Unexpected error injecting the config: %v", err)
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

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "EventType not found",
		},
		{
			Name:   "Get EventType error",
			Scheme: scheme.Scheme,
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.EventType); ok {
							return controllertesting.Handled, errors.New("test error getting the EventType")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the EventType",
		},
		{
			Name:   "EventType being deleted",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeDeletingEventType(),
			},
			WantEvent: []corev1.Event{events[eventTypeReconciled]},
		},
		{
			Name:   "Get Broker error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeEventType(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error getting broker")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting broker",
			WantEvent:  []corev1.Event{events[eventTypeReconcileFailed]},
		},
		{
			Name:   "Broker not ready",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeEventType(),
				makeBroker(),
			},
			WantErrMsg: `broker "` + eventTypeBroker + `" not ready`,
			WantEvent:  []corev1.Event{events[eventTypeReconcileFailed]},
		},
		{
			Name:   "EventType reconciliation success",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeEventType(),
				makeBrokerReady(),
			},
			WantEvent: []corev1.Event{events[eventTypeReconciled]},
			WantPresent: []runtime.Object{
				makeReadyEventType(),
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
			recorder:      recorder,
			logger:        zap.NewNop(),
		}
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, eventTypeName)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeEventType() *v1alpha1.EventType {
	return &v1alpha1.EventType{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "EventType",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      eventTypeName,
		},
		Spec: v1alpha1.EventTypeSpec{
			Broker: eventTypeBroker,
			Type:   eventTypeType,
		},
	}
}

func makeReadyEventType() *v1alpha1.EventType {
	t := makeEventType()
	t.Status.InitializeConditions()
	t.Status.MarkBrokerExists()
	t.Status.MarkBrokerReady()
	return t
}

func makeDeletingEventType() *v1alpha1.EventType {
	et := makeReadyEventType()
	et.DeletionTimestamp = &deletionTime
	return et
}

func makeBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      eventTypeBroker,
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{
				Provisioner: makeChannelProvisioner(),
			},
		},
	}
}

func makeBrokerReady() *v1alpha1.Broker {
	b := makeBroker()
	b.Status.InitializeConditions()
	b.Status.MarkTriggerChannelReady()
	b.Status.MarkFilterReady()
	b.Status.MarkIngressReady()
	b.Status.SetAddress("test-address")
	b.Status.MarkIngressChannelReady()
	b.Status.MarkIngressSubscriptionReady()
	return b
}

func makeChannelProvisioner() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "ClusterChannelProvisioner",
		Name:       "my-provisioner",
	}
}
