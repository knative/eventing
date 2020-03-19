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

package mtnamespace

import (
	"context"
	"testing"

	"knative.dev/pkg/configmap"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/reconciler/mtnamespace/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS                    = "test-namespace"
	brokerImagePullSecretName = "broker-image-pull-secret"
)

var (
	brokerGVR = schema.GroupVersionResource{
		Group:    "eventing.knative.dev",
		Version:  "v1alpha1",
		Resource: "brokers",
	}

	roleBindingGVR = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "rolebindings",
	}

	serviceAccountGVR = schema.GroupVersionResource{
		Version:  "v1",
		Resource: "serviceaccounts",
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	// Events
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created.")
	nsEvent := Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\"")

	// Object
	namespace := NewNamespace(testNS,
		WithNamespaceLabeled(resources.InjectionEnabledLabels()),
	)
	broker := resources.MakeBroker(namespace)

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
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
	}, {
		Name: "Namespace is labeled disabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionDisabledLabels())),
		},
		Key: testNS,
	}, {
		Name: "Namespace is deleted no resources",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
				WithNamespaceDeleted,
			),
		},
		Key: testNS,
		WantEvents: []string{
			nsEvent,
		},
	}, {
		Name: "Namespace enabled",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
	}, {
		Name: "Namespace enabled, broker exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			resources.MakeBroker(NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			)),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			nsEvent,
		},
	}, {
		Name: "Namespace enabled, broker exists with no label",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionDisabledLabels()),
			),
			&v1alpha1.Broker{
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
		return &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			namespaceLister: listers.GetNamespaceLister(),
			brokerLister:    listers.GetV1Beta1BrokerLister(),
		}
	}, false, logger))
}
