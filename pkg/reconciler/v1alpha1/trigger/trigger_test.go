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
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler/names"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	brokerName  = "test-broker"

	subscriberAPIVersion = "v1"
	subscriberKind       = "Service"
	subscriberName       = "subscriberName"
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
			Name:   "Get Trigger error",
			Scheme: scheme.Scheme,
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
			Name:   "Trigger being deleted",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeDeletingTrigger(),
			},
			WantEvent: []corev1.Event{events[triggerReconciled]},
		},
		{
			Name:   "Get Broker error",
			Scheme: scheme.Scheme,
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
			Name:   "Get Broker Trigger channel error",
			Scheme: scheme.Scheme,
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
			Name:   "Get Broker Ingress channel error",
			Scheme: scheme.Scheme,
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
			Name:   "Resolve subscriberURI error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
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
			Name:   "Create K8s Service error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*corev1.Service); ok {
							return controllertesting.Handled, errors.New("test error creating k8s service")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating k8s service",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name:   "Update K8s Service error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeDifferentK8sService(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*corev1.Service); ok {
							return controllertesting.Handled, errors.New("test error updating k8s service")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating k8s service",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name:   "Create Virtual Service error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*istiov1alpha3.VirtualService); ok {
							return controllertesting.Handled, errors.New("test error creating virtual service")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating virtual service",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name:   "Update Virtual Service error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeDifferentVirtualService(),
			},
			Objects: []runtime.Object{
				makeSubscriberServiceAsUnstructured(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*istiov1alpha3.VirtualService); ok {
							return controllertesting.Handled, errors.New("test error updating virtual service")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating virtual service",
			WantEvent:  []corev1.Event{events[triggerReconcileFailed]},
		},
		{
			Name:   "Create Subscription error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeVirtualService(),
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
			Name:   "Delete Subscription error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeVirtualService(),
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
			Name:   "Re-create Subscription error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeVirtualService(),
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
			Name:   "Update status error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeVirtualService(),
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
			Name:   "Trigger reconciliation success",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeTrigger(),
				makeBroker(),
				makeTriggerChannel(),
				makeK8sService(),
				makeVirtualService(),
				makeSameSubscription(),
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
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
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
	t.Status.InitializeConditions()
	t.Status.MarkBrokerExists()
	t.Status.SubscriberURI = fmt.Sprintf("http://%s.%s.svc.%s/", subscriberName, testNS, utils.GetClusterDomainName())
	t.Status.MarkKubernetesServiceExists()
	t.Status.MarkVirtualServiceExists()
	t.Status.MarkSubscribed()
	return t
}

func makeDeletingTrigger() *v1alpha1.Trigger {
	b := makeReadyTrigger()
	b.DeletionTimestamp = &deletionTime
	return b
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

func makeK8sService() *corev1.Service {
	return newK8sService(makeTrigger())
}

func makeDifferentK8sService() *corev1.Service {
	svc := makeK8sService()
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name: "http",
			Port: 9999,
		},
	}
	return svc
}

func makeVirtualService() *istiov1alpha3.VirtualService {
	return newVirtualService(makeTrigger(), makeK8sService())
}

func makeDifferentVirtualService() *istiov1alpha3.VirtualService {
	vsvc := makeVirtualService()
	vsvc.Spec.Hosts = []string{
		names.ServiceHostName("other_svc_name", "other_svc_namespace"),
	}
	return vsvc
}

func makeSameSubscription() *v1alpha1.Subscription {
	return makeSubscription(makeTrigger(), makeTriggerChannel(), makeTriggerChannel(), makeK8sService())
}

func makeDifferentSubscription() *v1alpha1.Subscription {
	return makeSubscription(makeTrigger(), makeTriggerChannel(), makeDifferentChannel(), makeK8sService())
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
