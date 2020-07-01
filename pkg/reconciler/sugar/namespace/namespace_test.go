/*
Copyright 2020 The Knative Authors

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
	"testing"

	"knative.dev/eventing/pkg/reconciler/sugar"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/reconciler/sugar/resources"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	namespacereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS = "test-namespace"
)

func init() {
	// Add types to scheme
	_ = v1beta1.AddToScheme(scheme.Scheme)
}

func TestEnabledByDefault(t *testing.T) {
	// Events
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created.")

	// Objects
	broker := resources.MakeBroker(testNS, resources.DefaultBrokerName)

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "Namespace is not labeled",
		Objects: []runtime.Object{
			NewNamespace(testNS),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
	}, {
		Name: "Namespace is labeled disabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionDisabledLabels())),
		},
		Key: testNS,
	}, {
		Name: "Namespace is deleted no resources",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
				WithNamespaceDeleted,
			),
		},
		Key: testNS,
	}, {
		Name: "Namespace enabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
			),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
	}, {
		Name: "Namespace enabled, broker exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
			),
			resources.MakeBroker(testNS, resources.DefaultBrokerName),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
	}, {
		Name: "Namespace enabled, broker exists with no label",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionDisabledLabels()),
			),
			&v1beta1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      resources.DefaultBrokerName,
				},
			},
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventingClientSet: fakeeventingclient.Get(ctx),
			isEnabled:         sugar.OnByDefault,
			brokerLister:      listers.GetV1Beta1BrokerLister(),
		}

		return namespacereconciler.NewReconciler(ctx, logger,
			fakekubeclient.Get(ctx), listers.GetNamespaceLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{SkipStatusUpdates: true})
	}, false, logger))
}

func TestDisabledByDefault(t *testing.T) {
	// Events
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created.")

	broker := resources.MakeBroker(testNS, resources.DefaultBrokerName)

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "Namespace is not labeled",
		Objects: []runtime.Object{
			NewNamespace(testNS),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		// When we're off by default, nothing happens when the label is missing.
	}, {
		Name: "Namespace is labeled disabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionDisabledLabels())),
		},
		Key: testNS,
	}, {
		Name: "Namespace is deleted no resources",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
				WithNamespaceDeleted,
			),
		},
		Key: testNS,
	}, {
		Name: "Namespace enabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
			),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
	}, {
		Name: "Namespace enabled, broker exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionEnabledLabels()),
			),
			resources.MakeBroker(testNS, resources.DefaultBrokerName),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
	}, {
		Name: "Namespace enabled, broker exists with no label",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(sugar.InjectionDisabledLabels()),
			),
			&v1beta1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      resources.DefaultBrokerName,
				},
			},
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventingClientSet: fakeeventingclient.Get(ctx),
			isEnabled:         sugar.OffByDefault,
			brokerLister:      listers.GetV1Beta1BrokerLister(),
		}

		return namespacereconciler.NewReconciler(ctx, logger,
			fakekubeclient.Get(ctx), listers.GetNamespaceLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{SkipStatusUpdates: true})
	}, false, logger))
}
