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
	//	"context"
	//	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/trigger/resources"
	brokerresources "github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	brokerName  = "test-broker"

	subscriberAPIVersion = "v1"
	subscriberKind       = "Service"
	subscriberName       = "subscriberName"
	subscriberURI        = "http://example.com/subscriber"
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
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
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
		}, { // TODO: there is a bug in the controller, it will query for ""
			//			Name: "trigger key not found ",
			//			Objects: []runtime.Object{
			//				reconciletesting.NewTrigger(triggerName, testNS),
			//			},
			//			Key:     "foo/incomplete",
			//			WantErr: true,
			//			WantEvents: []string{
			//				Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//			},
		}, {
			Name: "Broker not found",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"test-broker\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("DoesNotExist", "Broker does not exist"),
				),
			}},
		}, {
			Name: "Trigger being deleted",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerDeleted),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "No Broker Trigger Channel",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerChannelFailed", "Broker's Trigger channel not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Trigger channel"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "No Broker Ingress Channel",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "IngressChannelFailed", "Broker's Ingress channel not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Ingress channel"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "No Broker Filter Service",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerServiceFailed", "Broker's Filter service not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Filter service"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "Subscription Created, not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(makeServiceURI().String()),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(makeServiceURI().String()),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(makeServiceURI().String()),
				),
			}},
			WantCreates: []metav1.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription Created, not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(makeServiceURI().String()),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerSubscriberURI(makeServiceURI().String()),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(makeServiceURI().String()),
				),
			}},
		},
	}

	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			triggerLister:      listers.GetTriggerLister(),
			channelLister:      listers.GetChannelLister(),
			subscriptionLister: listers.GetSubscriptionLister(),
			brokerLister:       listers.GetBrokerLister(),
			serviceLister:      listers.GetK8sServiceLister(),
			tracker:            tracker.New(func(string) {}, 0),
		}

	}))
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

func newChannel(name string, labels map[string]string) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      name,
			Labels:    labels,
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
	labels := map[string]string{
		"eventing.knative.dev/broker":           brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	return newChannel(fmt.Sprintf("%s-broker", brokerName), labels)
}

func makeIngressChannel() *v1alpha1.Channel {
	labels := map[string]string{
		"eventing.knative.dev/broker":        brokerName,
		"eventing.knative.dev/brokerIngress": "true",
	}
	return newChannel(fmt.Sprintf("%s-broker-ingress", brokerName), labels)
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

func makeIngressSubscription() *v1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), makeTriggerChannel(), makeIngressChannel(), makeServiceURI())
}

func makeReadySubscription() *v1alpha1.Subscription {
	s := makeIngressSubscription()
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
