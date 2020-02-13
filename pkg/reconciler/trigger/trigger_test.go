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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
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
	triggerGeneration  = 7
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
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
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
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNamespace(testNS,
					reconciletesting.WithNamespaceLabeled(map[string]string{v1alpha1.InjectionAnnotation: injectionAnnotation})),
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
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: namespace \"test-namespace\" not found"),
			},
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
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for update namespaces"),
			},
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
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			triggerLister:   listers.GetTriggerLister(),
			brokerLister:    listers.GetBrokerLister(),
			namespaceLister: listers.GetNamespaceLister(),
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
			Subscriber: duckv1.Destination{
				Ref: &duckv1.KReference{
					Name:       subscriberName,
					Namespace:  testNS,
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

func makeEmptyDelivery() *eventingduckv1alpha1.DeliverySpec {
	return nil
}

func makeReadyBrokerNoTriggerChannel() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	return b
}

func makeReadyBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	b.Status.TriggerChannel = makeTriggerChannelRef()
	return b
}

func makeUnknownStatusBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.UnknownBrokerStatus()
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

func makeBrokerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "Broker",
		Namespace:  testNS,
		Name:       brokerName,
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
	return resources.NewSubscription(makeTrigger(), makeTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeEmptyDelivery())
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

func makeFalseStatusSubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.FalseSubscriptionStatus()
	return s
}

func makeFalseStatusCronJobSource() *sourcesv1alpha1.CronJobSource {
	return NewCronJobSource(cronJobSourceName, testNS, WithCronJobApiVersion(cronJobSourceAPIVersion), WithCronJobSourceSinkNotFound)
}

func makeUnknownStatusCronJobSource() *sourcesv1alpha1.CronJobSource {
	cjs := NewCronJobSource(cronJobSourceName, testNS, WithCronJobApiVersion(cronJobSourceAPIVersion))
	cjs.Status = *v1alpha1.TestHelper.UnknownCronJobSourceStatus()
	return cjs
}

func makeGenerationNotEqualCronJobSource() *sourcesv1alpha1.CronJobSource {
	c := makeFalseStatusCronJobSource()
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
