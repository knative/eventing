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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/trigger"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"
	brokerName  = "test-broker"

	subscriberURI = "http://example.com/subscriber/"

	injectionAnnotation = "enabled"
)

func init() {
	// Add types to scheme
	_ = v1beta1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	triggerKey := testNS + "/" + triggerName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "Trigger being deleted, nop",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerDeleted),
			},
			WantErr: false,
		}, {
			Name: "Non-default broker not found",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist or there is no matching BrokerClass for it`)),
			}},
			WantErr: false,
		}, {
			Name: "Default broker not found, with injection annotation enabled",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation)),
				reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{})),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "default" does not exist or there is no matching BrokerClass for it`)),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{v1beta1.InjectionAnnotation: injectionAnnotation})),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerNamespaceLabeled", "Trigger namespaced labeled for injection: %q", testNS),
			},
		}, {
			Name: "Default broker not found, with injection annotation enabled, namespace get fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation)),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("get", "namespaces"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "namespace \"test-namespace\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "default" does not exist or there is no matching BrokerClass for it`)),
			}},
		}, {
			Name: "Default broker not found, with injection annotation enabled, namespace label fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation)),
				reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{})),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "namespaces"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update namespaces"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{v1beta1.InjectionAnnotation: injectionAnnotation})),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "default" does not exist or there is no matching BrokerClass for it`)),
			}},
		}, {
			Name: "Default broker not found, with injection annotation enabled, trigger status update fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation)),
				reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{})),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "triggers"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", "Failed to update status for \"test-trigger\": inducing failure for update triggers"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{v1beta1.InjectionAnnotation: injectionAnnotation})),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "default" does not exist or there is no matching BrokerClass for it`)),
			}},
		}, {
			Name: "Default broker found, with injection annotation enabled",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyDefaultBroker(),
				reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation)),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventingClientSet: fakeeventingclient.Get(ctx),
			kubeClientSet:     fakekubeclient.Get(ctx),
			brokerLister:      listers.GetBrokerLister(),
			namespaceLister:   listers.GetNamespaceLister(),
		}
		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func makeBroker() *v1beta1.Broker {
	return &v1beta1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1beta1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      brokerName,
		},
		Spec: v1beta1.BrokerSpec{},
	}
}

func makeReadyBroker() *v1beta1.Broker {
	b := makeBroker()
	b.Status = *v1beta1.TestHelper.ReadyBrokerStatus()
	return b
}

func makeReadyDefaultBroker() *v1beta1.Broker {
	b := makeReadyBroker()
	b.Name = "default"
	return b
}
