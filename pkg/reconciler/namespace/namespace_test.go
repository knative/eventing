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
	"testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/system"

	"knative.dev/pkg/tracker"

	"github.com/knative/eventing/pkg/reconciler/namespace/resources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS = "test-namespace"
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
	saIngressEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountCreated", "ServiceAccount 'eventing-broker-ingress' created for the Broker")
	rbIngressEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'test-namespace/eventing-broker-ingress' created for the Broker")
	rbIngressConfigEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'knative-testing/eventing-broker-ingress-test-namespace' created for the Broker")
	saFilterEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountCreated", "ServiceAccount 'eventing-broker-filter' created for the Broker")
	rbFilterEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'test-namespace/eventing-broker-filter' created for the Broker")
	rbFilterConfigEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'knative-testing/eventing-broker-filter-test-namespace' created for the Broker")
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created.")
	nsEvent := Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\"")

	// Object
	broker := resources.MakeBroker(testNS)
	saIngress := resources.MakeServiceAccount(testNS, resources.IngressServiceAccountName)
	rbIngress := resources.MakeRoleBinding(resources.IngressRoleBindingName, testNS, resources.MakeServiceAccount(testNS, resources.IngressServiceAccountName), resources.IngressClusterRoleName)
	rbIngressConfig := resources.MakeRoleBinding(resources.ConfigRoleBindingName(resources.IngressServiceAccountName, testNS), system.Namespace(), resources.MakeServiceAccount(testNS, resources.IngressServiceAccountName), resources.ConfigClusterRoleName)
	saFilter := resources.MakeServiceAccount(testNS, resources.FilterServiceAccountName)
	rbFilter := resources.MakeRoleBinding(resources.FilterRoleBindingName, testNS, resources.MakeServiceAccount(testNS, resources.FilterServiceAccountName), resources.FilterClusterRoleName)
	rbFilterConfig := resources.MakeRoleBinding(resources.ConfigRoleBindingName(resources.FilterServiceAccountName, testNS), system.Namespace(), resources.MakeServiceAccount(testNS, resources.FilterServiceAccountName), resources.ConfigClusterRoleName)

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
		Key: testNS,
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
			saIngressEvent,
			rbIngressEvent,
			rbIngressConfigEvent,
			saFilterEvent,
			rbFilterEvent,
			rbFilterConfigEvent,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
			saIngress,
			rbIngress,
			rbIngressConfig,
			saFilter,
			rbFilter,
			rbFilterConfig,
		},
	}, {
		Name: "Namespace enabled, broker exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			resources.MakeBroker(testNS),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			saIngressEvent,
			rbIngressEvent,
			rbIngressConfigEvent,
			saFilterEvent,
			rbFilterEvent,
			rbFilterConfigEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			saIngress,
			rbIngress,
			rbIngressConfig,
			saFilter,
			rbFilter,
			rbFilterConfig,
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
	}, {
		Name: "Namespace enabled, ingress service account exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			saIngress,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			rbIngressEvent,
			rbIngressConfigEvent,
			saFilterEvent,
			rbFilterEvent,
			rbFilterConfigEvent,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
			rbIngress,
			rbIngressConfig,
			saFilter,
			rbFilter,
			rbFilterConfig,
		},
	}, {
		Name: "Namespace enabled, ingress role binding exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			rbIngress,
			rbIngressConfig,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			saIngressEvent,
			saFilterEvent,
			rbFilterEvent,
			rbFilterConfigEvent,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
			saIngress,
			saFilter,
			rbFilter,
			rbFilterConfig,
		},
	}, {
		Name: "Namespace enabled, filter service account exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			saFilter,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			saIngressEvent,
			rbIngressEvent,
			rbIngressConfigEvent,
			rbFilterEvent,
			rbFilterConfigEvent,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
			saIngress,
			rbIngress,
			rbIngressConfig,
			rbFilter,
			rbFilterConfig,
		},
	}, {
		Name: "Namespace enabled, filter role binding exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			rbFilter,
			rbFilterConfig,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			saIngressEvent,
			rbIngressEvent,
			rbIngressConfigEvent,
			saFilterEvent,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			broker,
			saIngress,
			rbIngress,
			rbIngressConfig,
			saFilter,
		},
	},
	// TODO: we need a existing default un-owned test.
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
			namespaceLister:      listers.GetNamespaceLister(),
			brokerLister:         listers.GetBrokerLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),
			roleBindingLister:    listers.GetRoleBindingLister(),
			tracker:              tracker.New(func(string) {}, 0),
		}
	}, false))
}
