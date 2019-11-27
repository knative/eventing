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
	"fmt"
	"net/url"
	"testing"

	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	"knative.dev/pkg/resolver"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	brokerresources "knative.dev/eventing/pkg/reconciler/broker/resources"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/pkg/reconciler/trigger/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "messaging.knative.dev/v1alpha1",
	}

	brokerRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	brokerDest = duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS

	subscriberGVK = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}
	subscriberAPIVersion = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)

	k8sServiceGVK = metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"
	brokerName  = "test-broker"

	subscriberGroup             = "serving.knative.dev"
	subscriberVersion           = "v1"
	subscriberKind              = "Service"
	subscriberName              = "subscriber-name"
	subscriberURI               = "http://example.com/subscriber/"
	subscriberURIReference      = "foo"
	subscriberResolvedTargetURI = "http://example.com/subscriber/foo"

	k8sServiceResolvedURI = "http://subscriber-name.test-namespace.svc.cluster.local/"

	dependencyAnnotation    = "{\"kind\":\"CronJobSource\",\"name\":\"test-cronjob-source\",\"apiVersion\":\"sources.eventing.knative.dev/v1alpha1\"}"
	cronJobSourceName       = "test-cronjob-source"
	cronJobSourceAPIVersion = "sources.eventing.knative.dev/v1alpha1"
	testSchedule            = "*/2 * * * *"
	testData                = "data"
	sinkName                = "testsink"

	injectionAnnotation = "enabled"

	currentGeneration  = 1
	outdatedGeneration = 0
)

var (
	trueVal = true

	subscriptionName = fmt.Sprintf("%s-%s-%s", brokerName, triggerName, triggerUID)
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
			//					reconciletesting.WithTriggerUID(triggerUID),
			//			},
			//			Key:     "foo/incomplete",
			//			WantErr: true,
			//			WantEvents: []string{
			//				Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//			},
		}, {
			Name: "Non-default broker not found",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"test-broker\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),

					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("DoesNotExist", "Broker does not exist"),
				),
			}},
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
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"default\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
					reconciletesting.WithTriggerBrokerFailed("DoesNotExist", "Broker does not exist"),
				),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{v1alpha1.InjectionAnnotation: injectionAnnotation})),
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
			WantErr: true,
			WantEvents: []string{
				// Only check if default broker is ready (not check other resources), so failed at the next step, check for filter service
				Eventf(corev1.EventTypeWarning, "TriggerServiceFailed", "Broker's Filter service not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Filter service"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, "default",
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithInjectionAnnotation(injectionAnnotation),
				),
			}},
		}, {
			Name: "Broker get failure",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"test-broker\" not found"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("get", "brokers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
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
					reconciletesting.WithTriggerUID(triggerUID),
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
				makeReadyBrokerNoTriggerChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
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
					reconciletesting.WithTriggerUID(triggerUID),
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
				makeReadyBrokerNoIngressChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
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
					reconciletesting.WithTriggerUID(triggerUID),
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
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
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
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "Subscription not owned by Trigger",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeIngressSubscriptionNotOwnedByTrigger(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: trigger %q does not own subscription %q", triggerName, subscriptionName)},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", fmt.Sprintf("trigger %q does not own subscription %q", triggerName, subscriptionName)),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
				),
			}},
		}, {
			Name: "Subscription create fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for create subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription delete fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Delete Trigger's subscription failed: inducing failure for delete subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for delete subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for delete subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
		}, {
			Name: "Subscription create after delete fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for create subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription updated works",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription Created, not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeSubscriberAddressableAsUnstructured(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(subscriberGVK, subscriberName),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists and URI",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeSubscriberAddressableAsUnstructured(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, subscriberURIReference),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists kubernetes Service",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeSubscriberKubernetesServiceAsUnstructured(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(k8sServiceGVK, subscriberName),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(k8sServiceGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(k8sServiceResolvedURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref doesn't exist",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(subscriberGVK, subscriberName),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", `Trigger reconciliation failed: failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
				),
			}},
		}, {
			Name: "Subscription not ready, trigger marked not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeNotReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: test induced [error]"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Subscription ready, trigger marked ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
				Eventf(corev1.EventTypeNormal, "TriggerReadinessChanged", `Trigger "test-trigger" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Dependency doesn't exist",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: propagating dependency readiness: getting the dependency: cronjobsources.sources.eventing.knative.dev \"test-cronjob-source\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyUnknown("DependencyDoesNotExist", "Dependency does not exist: cronjobsources.sources.eventing.knative.dev \"test-cronjob-source\" not found"),
				),
			}},
		}, {
			Name: "Dependency not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				makeNotReadyCronJobSource(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyFailed("DependencyNotReady", "Dependency is not ready: "),
				),
			}},
		}, {
			Name: "Dependency generation not equal",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				makeGenerationNotEqualCronJobSource(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration))),
			}},
		},
		{
			Name: "Dependency ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				makeReadyCronJobSource(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
				Eventf(corev1.EventTypeNormal, "TriggerReadinessChanged", `Trigger "test-trigger" became ready`)},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithDependencyAnnotation(dependencyAnnotation),
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
					reconciletesting.WithTriggerSubscriberResolvedSucceeded(),
					reconciletesting.WithTriggerDependencyReady(),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			triggerLister:      listers.GetTriggerLister(),
			subscriptionLister: listers.GetSubscriptionLister(),
			brokerLister:       listers.GetBrokerLister(),
			serviceLister:      listers.GetK8sServiceLister(),
			namespaceLister:    listers.GetNamespaceLister(),
			tracker:            tracker.New(func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1a1addr.Get, func(types.NamespacedName) {}, 0),
			kresourceTracker:   duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		}
	},
		false,
		logger,
	))
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
			UID:       triggerUID,
		},
		Spec: v1alpha1.TriggerSpec{
			Broker: brokerName,
			Filter: &v1alpha1.TriggerFilter{
				DeprecatedSourceAndType: &v1alpha1.TriggerFilterSourceAndType{
					Source: "Any",
					Type:   "Any",
				},
			},
			Subscriber: &duckv1.Destination{
				Ref: &corev1.ObjectReference{
					Name:       subscriberName,
					Kind:       subscriberKind,
					APIVersion: subscriberAPIVersion,
				},
			},
		},
	}
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
		Spec: v1alpha1.BrokerSpec{},
	}
}

func makeReadyBrokerNoTriggerChannel() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	return b
}

func makeReadyBrokerNoIngressChannel() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	b.Status.TriggerChannel = makeTriggerChannelRef()
	return b
}

func makeReadyBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	b.Status.TriggerChannel = makeTriggerChannelRef()
	b.Status.IngressChannel = makeIngressChannelRef()
	return b
}

func makeReadyDefaultBroker() *v1alpha1.Broker {
	b := makeReadyBroker()
	b.Name = "default"
	return b
}

func makeTriggerChannelRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "Channel",
		Namespace:  testNS,
		Name:       fmt.Sprintf("%s-kn-trigger", brokerName),
	}
}

func makeIngressChannelRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "Channel",
		Namespace:  testNS,
		Name:       fmt.Sprintf("%s-kn-ingress", brokerName),
	}
}

func makeSubscriberAddressableAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": subscriberURI,
				},
			},
		},
	}
}

func makeSubscriberKubernetesServiceAsUnstructured() *unstructured.Unstructured {
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
		Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
	}
}

func makeIngressSubscription() *messagingv1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), makeTriggerChannelRef(), makeIngressChannelRef(), makeServiceURI())
}

func makeIngressSubscriptionNotOwnedByTrigger() *messagingv1alpha1.Subscription {
	sub := makeIngressSubscription()
	sub.OwnerReferences = []metav1.OwnerReference{}
	return sub
}

// Just so we can test subscription updates
func makeDifferentReadySubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Spec.Subscriber.URI = apis.HTTP("different.example.com")
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeReadySubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeNotReadySubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.NotReadySubscriptionStatus()
	return s
}

func makeNotReadyCronJobSource() *sourcesv1alpha1.CronJobSource {
	return NewCronJobSource(cronJobSourceName, testNS, WithCronJobApiVersion(cronJobSourceAPIVersion), WithCronJobSourceSinkNotFound)
}

func makeGenerationNotEqualCronJobSource() *sourcesv1alpha1.CronJobSource {
	c := makeNotReadyCronJobSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyCronJobSource() *sourcesv1alpha1.CronJobSource {
	return NewCronJobSource(cronJobSourceName, testNS,
		WithCronJobApiVersion(cronJobSourceAPIVersion),
		WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
			Schedule: testSchedule,
			Data:     testData,
			Sink:     &brokerDest,
		}),
		WithInitCronJobSourceConditions,
		WithValidCronJobSourceSchedule,
		WithValidCronJobSourceResources,
		WithCronJobSourceDeployed,
		WithCronJobSourceEventType,
		WithCronJobSourceSink(sinkURI),
	)
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
