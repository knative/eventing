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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/reconciler/namespace/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
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
	cmpEvent := Eventf(corev1.EventTypeNormal, "ConfigMapPropagationCreated", "Default ConfigMapPropagation: eventing created")
	saIngressEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountCreated", "ServiceAccount 'eventing-broker-ingress' created for the Broker")
	rbIngressEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'test-namespace/eventing-broker-ingress' created for the Broker")
	saFilterEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountCreated", "ServiceAccount 'eventing-broker-filter' created for the Broker")
	rbFilterEvent := Eventf(corev1.EventTypeNormal, "BrokerServiceAccountRBACCreated", "RoleBinding 'test-namespace/eventing-broker-filter' created for the Broker")
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker created.")
	nsEvent := Eventf(corev1.EventTypeNormal, "NamespaceReconciled", "Namespace reconciled: \"test-namespace\"")
	nsEventFailure := Eventf(corev1.EventTypeWarning, "NamespaceReconcileFailure", "Failed to reconcile Namespace: broker ingress: Error copying secret knative-testing/broker-image-pull-secret => test-namespace/eventing-broker-ingress : secrets \"broker-image-pull-secret\" not found")

	secretEventFilter := Eventf(corev1.EventTypeNormal, "SecretCopied", "Secret copied into namespace knative-testing/broker-image-pull-secret => test-namespace/eventing-broker-filter")
	secretEventIngress := Eventf(corev1.EventTypeNormal, "SecretCopied", "Secret copied into namespace knative-testing/broker-image-pull-secret => test-namespace/eventing-broker-ingress")
	secretEventFailure := Eventf(corev1.EventTypeWarning, "SecretCopyFailure", "Error copying secret knative-testing/broker-image-pull-secret => test-namespace/eventing-broker-ingress : secrets \"broker-image-pull-secret\" not found")

	// Patches
	ingressPatch := createPatch(testNS, "eventing-broker-ingress")
	filterPatch := createPatch(testNS, "eventing-broker-filter")

	// Object
	secret := resources.MakeSecret(brokerImagePullSecretName)
	broker := resources.MakeBroker(testNS)
	saIngress := resources.MakeServiceAccount(testNS, resources.IngressServiceAccountName)
	rbIngress := resources.MakeRoleBinding(resources.IngressRoleBindingName, testNS, resources.MakeServiceAccount(testNS, resources.IngressServiceAccountName), resources.IngressClusterRoleName)
	saFilter := resources.MakeServiceAccount(testNS, resources.FilterServiceAccountName)
	rbFilter := resources.MakeRoleBinding(resources.FilterRoleBindingName, testNS, resources.MakeServiceAccount(testNS, resources.FilterServiceAccountName), resources.FilterClusterRoleName)
	configMapPropagation := resources.MakeConfigMapPropagation(testNS)

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
			cmpEvent,
			saIngressEvent,
			rbIngressEvent,
			secretEventIngress,
			saFilterEvent,
			rbFilterEvent,
			secretEventFilter,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			broker,
			secret,
			saIngress,
			rbIngress,
			secret,
			saFilter,
			rbFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
		},
	}, {
		Name: "Namespace enabled - secret copy fails",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "secrets"),
		},
		WantEvents: []string{
			cmpEvent,
			saIngressEvent,
			rbIngressEvent,
			secretEventFailure,
			nsEventFailure,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			saIngress,
			rbIngress,
		},
	}, {
		Name: "Namespace enabled - configmappropagation fails",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "configmappropagations"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "NamespaceReconcileFailure", "Failed to reconcile Namespace: configMapPropagation: inducing failure for create configmappropagations"),
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
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
			cmpEvent,
			saIngressEvent,
			rbIngressEvent,
			secretEventIngress,
			saFilterEvent,
			rbFilterEvent,
			secretEventFilter,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			secret,
			saIngress,
			rbIngress,
			secret,
			saFilter,
			rbFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
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
			cmpEvent,
			rbIngressEvent,
			secretEventIngress,
			saFilterEvent,
			rbFilterEvent,
			secretEventFilter,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			broker,
			secret,
			rbIngress,
			secret,
			saFilter,
			rbFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
		},
	}, {
		Name: "Namespace enabled, ingress role binding exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			rbIngress,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			cmpEvent,
			saIngressEvent,
			secretEventIngress,
			saFilterEvent,
			rbFilterEvent,
			secretEventFilter,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			broker,
			secret,
			saIngress,
			secret,
			saFilter,
			rbFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
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
			cmpEvent,
			saIngressEvent,
			rbIngressEvent,
			secretEventIngress,
			rbFilterEvent,
			secretEventFilter,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			broker,
			secret,
			saIngress,
			rbIngress,
			secret,
			rbFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
		},
	}, {
		Name: "Namespace enabled, filter role binding exists",
		Objects: []runtime.Object{
			NewNamespace(testNS,
				WithNamespaceLabeled(resources.InjectionEnabledLabels()),
			),
			rbFilter,
		},
		Key:                     testNS,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			cmpEvent,
			saIngressEvent,
			rbIngressEvent,
			secretEventIngress,
			saFilterEvent,
			secretEventFilter,
			brokerEvent,
			nsEvent,
		},
		WantCreates: []runtime.Object{
			configMapPropagation,
			broker,
			secret,
			saIngress,
			rbIngress,
			secret,
			saFilter,
			secret,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			ingressPatch,
			filterPatch,
		},
	},
	// TODO: we need a existing default un-owned test.
	}

	logger := logtesting.TestLogger(t)
	// used to determine which test we are on
	testNum := 0
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {

		r := &Reconciler{
			Base:                       reconciler.NewBase(ctx, controllerAgentName, cmw),
			namespaceLister:            listers.GetNamespaceLister(),
			brokerLister:               listers.GetBrokerLister(),
			serviceAccountLister:       listers.GetServiceAccountLister(),
			roleBindingLister:          listers.GetRoleBindingLister(),
			configMapPropagationLister: listers.GetConfigMapPropagationLister(),
			brokerPullSecretName:       brokerImagePullSecretName,
			tracker:                    tracker.New(func(types.NamespacedName) {}, 0),
		}

		// only create secret in required tests
		createSecretTests := []string{"Namespace enabled", "Namespace enabled, broker exists",
			"Namespace enabled, ingress service account exists", "Namespace enabled, ingress role binding exists",
			"Namespace enabled, filter service account exists", "Namespace enabled, filter role binding exists"}

		for _, theTest := range createSecretTests {
			if theTest == table[testNum].Name {
				// create the required secret in knative-eventing to be copied into required namespaces
				tgtNSSecrets := r.KubeClientSet.CoreV1().Secrets(system.Namespace())
				tgtNSSecrets.Create(resources.MakeSecret(brokerImagePullSecretName))
				break
			}
		}
		testNum++
		return r
	}, false, logger))
}

func createPatch(namespace string, name string) clientgotesting.PatchActionImpl {
	patch := clientgotesting.PatchActionImpl{}
	patch.Namespace = namespace
	patch.Name = name
	patch.Patch = []byte(`{"imagePullSecrets":[{"name":"` + brokerImagePullSecretName + `"}]}`)
	return patch
}
