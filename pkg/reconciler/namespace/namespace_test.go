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
	"testing"

	"github.com/knative/pkg/tracker"

	"github.com/knative/eventing/pkg/reconciler/namespace/resources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
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

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	eventingInformer := eventinginformers.NewSharedInformerFactory(eventingClient, 0)

	namespaceInformer := kubeInformer.Core().V1().Namespaces()
	serviceAccountInformer := kubeInformer.Core().V1().ServiceAccounts()
	roleBindingInformer := kubeInformer.Rbac().V1().RoleBindings()
	brokerInformer := eventingInformer.Eventing().V1alpha1().Brokers()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	}, namespaceInformer, serviceAccountInformer, roleBindingInformer, brokerInformer)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func TestAllCases(t *testing.T) {
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
			Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\""),
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
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountCreated", "Service account created for the Broker 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountRBACCreated", "Service account RBAC created for the Broker Filter 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created."),
			Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\""),
		},
		WantCreates: []metav1.Object{
			resources.MakeBroker(testNS),
			resources.MakeServiceAccount(testNS),
			resources.MakeRoleBinding(resources.MakeServiceAccount(testNS)),
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
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountCreated", "Service account created for the Broker 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountRBACCreated", "Service account RBAC created for the Broker Filter 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\""),
		},
		WantCreates: []metav1.Object{
			resources.MakeServiceAccount(testNS),
			resources.MakeRoleBinding(resources.MakeServiceAccount(testNS)),
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
		Name: "Namespace enabled, service account exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			resources.MakeServiceAccount(testNS),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountRBACCreated", "Service account RBAC created for the Broker Filter 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created."),
			Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\""),
		},
		WantCreates: []metav1.Object{
			resources.MakeBroker(testNS),
			resources.MakeRoleBinding(resources.MakeServiceAccount(testNS)),
		},
	}, {
		Name: "Namespace enabled, role binding exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			resources.MakeRoleBinding(resources.MakeServiceAccount(testNS)),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "BrokerFilterServiceAccountCreated", "Service account created for the Broker 'eventing-broker-filter'"),
			Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created."),
			Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\""),
		},
		WantCreates: []metav1.Object{
			resources.MakeBroker(testNS),
			resources.MakeServiceAccount(testNS),
		},
	},
	// TODO: we need a existing default un-owned test.
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                 reconciler.NewBase(opt, controllerAgentName),
			namespaceLister:      listers.GetNamespaceLister(),
			brokerLister:         listers.GetBrokerLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),
			roleBindingLister:    listers.GetRoleBindingLister(),
			tracker:              tracker.New(func(string) {}, 0),
		}

	},
	false,
	))
}
