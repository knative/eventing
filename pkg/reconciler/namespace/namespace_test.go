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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	xxxs "sigs.k8s.io/controller-runtime/pkg/client"
	xxx "sigs.k8s.io/controller-runtime/pkg/client/fake"
	xx "sigs.k8s.io/controller-runtime/pkg/handler"
	x "sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	kubeinformers "k8s.io/client-go/informers"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"
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
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	namespaceInformer := kubeInformer.Core().V1().Namespaces()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	}, namespaceInformer)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func TestAllCases(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
			//}, { // TODO: there is a bug in the controller, it will query for ""
			//	Name: "incomplete subscription",
			//	Objects: []runtime.Object{
			//		NewSubscription(subscriptionName, testNS),
			//	},
			//	Key:     "foo/incomplete",
			//	WantErr: true,
			//	WantEvents: []string{
			//		Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//	},
		},
		//{
		//	Name:   "Namespace.Get fails",
		//	Scheme: scheme.Scheme,
		//	Mocks: controllertesting.Mocks{
		//		MockGets: []controllertesting.MockGet{
		//			func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//				if _, ok := obj.(*corev1.Namespace); ok {
		//					return controllertesting.Handled, errors.New("test error getting the NS")
		//				}
		//				return controllertesting.Unhandled, nil
		//			},
		//		},
		//	},
		//	WantErrMsg: "test error getting the NS",
		//},
		//{
		//	Name:   "Namespace is not labeled",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(nil),
		//	},
		//	WantAbsent: []runtime.Object{
		//		makeBroker(),
		//	},
		//},
		//{
		//	Name:   "Namespace is labeled disabled",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(&disabled),
		//	},
		//	WantAbsent: []runtime.Object{
		//		makeBroker(),
		//	},
		//},
		//{
		//	Name:   "Namespace is being deleted",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeDeletingNamespace(),
		//	},
		//	WantAbsent: []runtime.Object{
		//		makeBroker(),
		//	},
		//},
		//{
		//	Name:   "Broker.Get fails",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(&enabled),
		//	},
		//	Mocks: controllertesting.Mocks{
		//		MockGets: []controllertesting.MockGet{
		//			func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//				if _, ok := obj.(*v1alpha1.Broker); ok {
		//					return controllertesting.Handled, errors.New("test error getting the Broker")
		//				}
		//				return controllertesting.Unhandled, nil
		//			},
		//		},
		//	},
		//	WantErrMsg: "test error getting the Broker",
		//	WantAbsent: []runtime.Object{
		//		makeBroker(),
		//	},
		//	WantEvent: []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		//},
		//{
		//	Name:   "Broker Found",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(&enabled),
		//		makeBroker(),
		//	},
		//	WantEvent: []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		//},
		//{
		//	Name:   "Broker.Create fails",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(&enabled),
		//	},
		//	Mocks: controllertesting.Mocks{
		//		MockCreates: []controllertesting.MockCreate{
		//			func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//				if _, ok := obj.(*v1alpha1.Broker); ok {
		//					return controllertesting.Handled, errors.New("test error creating the Broker")
		//				}
		//				return controllertesting.Unhandled, nil
		//			},
		//		},
		//	},
		//	WantErrMsg: "test error creating the Broker",
		//	WantEvent:  []corev1.Event{events[serviceAccountCreated], events[serviceAccountRBACCreated]},
		//},
		//{
		//	Name:   "Broker created",
		//	Scheme: scheme.Scheme,
		//	InitialState: []runtime.Object{
		//		makeNamespace(&enabled),
		//	},
		//	WantPresent: []runtime.Object{
		//		makeBroker(),
		//	},
		//	WantEvent: []corev1.Event{
		//		events[serviceAccountCreated],
		//		events[serviceAccountRBACCreated],
		//		events[brokerCreated]},
		//},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}))

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
