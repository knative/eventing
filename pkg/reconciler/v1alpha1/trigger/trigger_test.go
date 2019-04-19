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

package trigger

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	brokerresources "github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/trigger/resources"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	brokerName  = "test-broker"

	subscriberAPIVersion = "v1"
	subscriberKind       = "Service"
	subscriberName       = "subscriberName"

	continueToken = "continueToken"
)

var (
	trueVal = true
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// Map of events to set test cases' expectations easier.
	events = map[string]corev1.Event{
		triggerReconciled:         {Reason: triggerReconciled, Type: corev1.EventTypeNormal},
		triggerUpdateStatusFailed: {Reason: triggerUpdateStatusFailed, Type: corev1.EventTypeWarning},
		triggerReconcileFailed:    {Reason: triggerReconcileFailed, Type: corev1.EventTypeWarning},
		subscriptionDeleteFailed:  {Reason: subscriptionDeleteFailed, Type: corev1.EventTypeWarning},
		subscriptionCreateFailed:  {Reason: subscriptionCreateFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = istiov1alpha3.AddToScheme(scheme.Scheme)
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
			Name: "Trigger not found",
		},
		{
			Name: "Get Trigger error",
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Trigger); ok {
							return controllertesting.Handled, errors.New("test error getting the Trigger")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the Trigger",
		},
		{
			Name: "Trigger being deleted",
			InitialState: []runtime.Object{
				makeDeletingTrigger(),
			},
			WantEvent: []corev1.Event{events[triggerReconciled]},
		},
		{
			Name: "Get Broker error",
			InitialState: []runtime.Object{
				makeTrigger(),
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
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Get Broker Trigger channel error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, opts *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
						// Only match the Trigger Channel labels.
						ls := labels.FormatLabels(broker.TriggerChannelLabels(makeBroker()))
						l, _ := labels.ConvertSelectorToLabelsMap(ls)

						if _, ok := list.(*v1alpha1.ChannelList); ok && opts.LabelSelector.Matches(l) {
							return controllertesting.Handled, errors.New("test error getting broker's Trigger channel")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting broker's Trigger channel",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Broker Trigger channel not found",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
			},
			WantErrMsg: ` "" not found`,
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Get Broker Ingress channel error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, opts *client.ListOptions, list runtime.Object) (handled controllertesting.MockHandled, e error) {
						// Only match the Ingress Channel labels.
						ls := labels.FormatLabels(broker.IngressChannelLabels(makeBroker()))
						l, _ := labels.ConvertSelectorToLabelsMap(ls)

						if _, ok := list.(*v1alpha1.ChannelList); ok && opts.LabelSelector.Matches(l) {
							return controllertesting.Handled, errors.New("test error getting broker's Ingress channel")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting broker's Ingress channel",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Broker Ingress channel not found",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
			},
			WantErrMsg: ` "" not found`,
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Broker Filter Service not found",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
			},
			WantErrMsg: ` "" not found`,
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Get Broker Filter Service error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, opts *client.ListOptions, list runtime.Object) (handled controllertesting.MockHandled, e error) {
						if _, ok := list.(*corev1.ServiceList); ok {
							return controllertesting.Handled, errors.New("test error getting Broker's filter Service")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting Broker's filter Service",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Resolve subscriberURI error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
			},
			DynamicMocks: controllertesting.DynamicMocks{
				MockGets: []controllertesting.MockDynamicGet{
					func(ctx *controllertesting.MockDynamicContext, name string, options metav1.GetOptions, subresources ...string) (handled controllertesting.MockHandled, i *unstructured.Unstructured, e error) {
						if ctx.Resource.Group == "" && ctx.Resource.Version == "v1" && ctx.Resource.Resource == "services" {

							return controllertesting.Handled, nil, errors.New("test error resolving subscriber URI")
						}
						return controllertesting.Unhandled, nil, nil
					},
				},
			},
			WantErrMsg: "test error resolving subscriber URI",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Get Subscription error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := list.(*v1alpha1.SubscriptionList); ok {
							return controllertesting.Handled, errors.New("test error listing subscription")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error listing subscription",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Create Subscription error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Subscription); ok {
							return controllertesting.Handled, errors.New("test error creating subscription")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating subscription",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name: "Delete Subscription error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
				makeDifferentSubscription(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockDeletes: []controllertesting.MockDelete{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Subscription); ok {
							return controllertesting.Handled, errors.New("test error deleting subscription")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error deleting subscription",
			WantEvent:  []corev1.Event{events[subscriptionDeleteFailed], events[triggerReconcileFailed]},
		},
		{
			Name: "Re-create Subscription error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
				makeDifferentSubscription(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Subscription); ok {
							return controllertesting.Handled, errors.New("test error re-creating subscription")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error re-creating subscription",
			WantEvent:  []corev1.Event{events[subscriptionCreateFailed], events[triggerReconcileFailed]},
		},
		{
			Name: "Update status error",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
				makeSameSubscription(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: []controllertesting.MockStatusUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Trigger); ok {
							return controllertesting.Handled, errors.New("test error updating trigger status")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating trigger status",
			WantEvent:  []corev1.Event{events[triggerReconciled], events[triggerUpdateStatusFailed]},
		},
		{
			Name: "Trigger reconciliation success",
			InitialState: []runtime.Object{
				makeTrigger(),
				makeReadyBroker(),
				makeTriggerChannel(),
				makeBrokerFilterService(),
				makeReadySubscription(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			WantEvent: []corev1.Event{events[triggerReconciled]},
			WantPresent: []runtime.Object{
				makeReadyTrigger(),
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
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, triggerName)
		tc.IgnoreTimes = true
		tc.Scheme = scheme.Scheme
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func TestMapBrokerToTriggers(t *testing.T) {
	testCases := map[string]struct {
		initialState []runtime.Object
		mocks        controllertesting.Mocks
		expected     []reconcile.Request
	}{
		"List error": {
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test induced error")
					},
				},
			},
			expected: []reconcile.Request{},
		},
		"One Trigger": {
			initialState: []runtime.Object{
				makeTrigger(),
			},
			expected: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: testNS,
						Name:      triggerName,
					},
				},
			},
		},
		"Only from this namespace": {
			initialState: []runtime.Object{
				makeTriggerWithNamespaceAndName(testNS, "one"),
				makeTriggerWithNamespaceAndName("some-other-namespace", "will-be-ignored"),
				makeTriggerWithNamespaceAndName(testNS, "two"),
			},
			expected: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: testNS,
						Name:      "one",
					},
				},
				{
					NamespacedName: types.NamespacedName{
						Namespace: testNS,
						Name:      "two",
					},
				},
			},
		},
		"Follows pagination": {
			initialState: []runtime.Object{
				makeTrigger(),
			},
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(innerClient client.Client, ctx context.Context, opts *client.ListOptions, list runtime.Object) (handled controllertesting.MockHandled, e error) {
						// The first request won't have a continue token. Add it and immediately
						// return. The subsequent request will have the token, remove it and send
						// the request to the inner client.
						tl := list.(*v1alpha1.TriggerList)
						if opts.Raw.Continue != continueToken {
							tl.Continue = continueToken
							return controllertesting.Handled, nil
						} else {
							tl.Continue = ""
							return controllertesting.Handled, innerClient.List(ctx, opts, list)
						}
					},
				},
			},
			expected: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: testNS,
						Name:      triggerName,
					},
				},
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c := (&controllertesting.TestCase{
				Scheme:       scheme.Scheme,
				InitialState: tc.initialState,
				Mocks:        tc.mocks,
			}).GetClient()

			b := &mapBrokerToTriggers{
				// client and logger are the only fields that are used by the Map function.
				r: &reconciler{
					client: c,
					logger: zap.NewNop(),
				},
			}
			o := handler.MapObject{
				Meta: &metav1.ObjectMeta{
					Namespace: testNS,
					Name:      brokerName,
				},
			}
			actual := b.Map(o)
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("Unexpected results (-want +got): %s", diff)
			}
		})
	}
}

func makeTrigger() *v1alpha1.Trigger {
	return &v1alpha1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
		},
		Spec: v1alpha1.TriggerSpec{
			Broker: brokerName,
			Filter: &v1alpha1.TriggerFilter{
				SourceAndType: &v1alpha1.TriggerFilterSourceAndType{
					Source: "Any",
					Type:   "Any",
				},
			},
			Subscriber: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					Name:       subscriberName,
					Kind:       subscriberKind,
					APIVersion: subscriberAPIVersion,
				},
			},
		},
	}
}

func makeReadyTrigger() *v1alpha1.Trigger {
	t := makeTrigger()
	t.Status = *v1alpha1.TestHelper.ReadyTriggerStatus()
	t.Status.SubscriberURI = fmt.Sprintf("http://%s.%s.svc.%s/", subscriberName, testNS, utils.GetClusterDomainName())
	return t
}

func makeDeletingTrigger() *v1alpha1.Trigger {
	b := makeReadyTrigger()
	b.DeletionTimestamp = &deletionTime
	return b
}

func makeTriggerWithNamespaceAndName(namespace, name string) *v1alpha1.Trigger {
	t := makeTrigger()
	t.Namespace = namespace
	t.Name = name
	return t
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
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{
				Provisioner: makeChannelProvisioner(),
			},
		},
	}
}

func makeReadyBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	return b
}

func makeChannelProvisioner() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "ClusterChannelProvisioner",
		Name:       "my-provisioner",
	}
}

func newChannel(name string) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      name,
			Labels: map[string]string{
				"eventing.knative.dev/broker":           brokerName,
				"eventing.knative.dev/brokerEverything": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReference(),
			},
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: makeChannelProvisioner(),
		},
		Status: v1alpha1.ChannelStatus{
			Address: duckv1alpha1.Addressable{
				Hostname: "any-non-empty-string",
			},
		},
	}
}

func makeTriggerChannel() *v1alpha1.Channel {
	return newChannel(fmt.Sprintf("%s-broker", brokerName))
}

func makeDifferentChannel() *v1alpha1.Channel {
	return newChannel(fmt.Sprintf("%s-broker-different", brokerName))
}

func makeSubscriberServiceAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
		},
	}
}

func makeBrokerFilterService() *corev1.Service {
	return brokerresources.MakeFilterService(makeBroker())
}

func makeServiceURI() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", makeBrokerFilterService().Name, testNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/triggers/%s/%s", testNS, triggerName),
	}
}

func makeSameSubscription() *v1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), makeTriggerChannel(), makeTriggerChannel(), makeServiceURI())
}

func makeDifferentSubscription() *v1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), makeTriggerChannel(), makeDifferentChannel(), makeServiceURI())
}

func makeReadySubscription() *v1alpha1.Subscription {
	s := makeSameSubscription()
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func getOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "Broker",
		Name:               brokerName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}
