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

package mtbroker

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/mtbroker/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	rtv1alpha1 "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS   = "knative-testing"
	testNS     = "test-namespace"
	brokerName = "test-broker"

	configMapName = "test-configmap"

	triggerName     = "test-trigger"
	triggerUID      = "test-trigger-uid"
	triggerNameLong = "test-trigger-name-is-a-long-name"
	triggerUIDLong  = "cafed00d-cafed00d-cafed00d-cafed00d-cafed00d"

	subscriberURI     = "http://example.com/subscriber/"
	subscriberKind    = "Service"
	subscriberName    = "subscriber-name"
	subscriberGroup   = "serving.knative.dev"
	subscriberVersion = "v1"

	pingSourceName              = "test-ping-source"
	testSchedule                = "*/2 * * * *"
	testData                    = "data"
	sinkName                    = "testsink"
	dependencyAnnotation        = "{\"kind\":\"PingSource\",\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1alpha2\"}"
	subscriberURIReference      = "foo"
	subscriberResolvedTargetURI = "http://example.com/subscriber/foo"

	k8sServiceResolvedURI = "http://subscriber-name.test-namespace.svc.cluster.local/"
	currentGeneration     = 1
	outdatedGeneration    = 0

	finalizerName = "brokers.eventing.knative.dev"

	imcSpec = `
apiVersion: "messaging.knative.dev/v1"
kind: "InMemoryChannel"
`
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	triggerChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())

	filterServiceName  = "broker-filter"
	ingressServiceName = "broker-ingress"

	subscriptionName = fmt.Sprintf("%s-%s-%s", brokerName, triggerName, triggerUID)

	subscriberAPIVersion = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)
	subscriberGVK        = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}
	k8sServiceGVK = metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
	brokerDestv1 = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
		},
	}
	sinkDNS               = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI               = "http://" + sinkDNS
	finalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`)

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, systemNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/%s/%s", testNS, brokerName),
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
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
			Name: "Broker not found",
			Key:  testKey,
		}, {
			Name: "Broker is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerDeletionTimestamp),
				imcConfigMap(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "test-namespace/test-broker"`),
			},
		}, {
			Name: "nil config",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "failed to find channelTemplate"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: failed to find channelTemplate")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "nil config, missing name",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "ConfigMap", APIVersion: "v1"}),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": missing field(s): spec.config.name`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "ConfigMap", APIVersion: "v1"}),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.Config name and namespace are required")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Config not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("ChannelTemplateFailed", `Error on setting up the ChannelTemplate: configmaps "test-configmap" not found`)),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", `configmaps "test-configmap" not found`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
			},
			WantCreates: []runtime.Object{
				createChannel(testNS, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("ChannelFailure", "inducing failure for create inmemorychannels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "inmemorychannels"),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "Failed to reconcile trigger channel: %v", "inducing failure for create inmemorychannels"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create no address",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
			},
			WantCreates: []runtime.Object{
				createChannel(testNS, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
		}, {
			Name: "Trigger Channel.Create no host in the url",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannelNoHostInUrl(testNS),
				imcConfigMap(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "nil config, not a configmap",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "Deployment", APIVersion: "v1", Name: "dummy"}),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", `Broker.Spec.Config configuration not supported, only [kind: ConfigMap, apiVersion: v1]`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "Deployment", APIVersion: "v1", Name: "dummy"}),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.Config configuration not supported, only [kind: ConfigMap, apiVersion: v1]")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Trigger Channel is not yet Addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
				createChannel(testNS, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
		}, {
			Name: "Trigger Channel endpoints fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
				createChannel(testNS, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterFailed("ServiceFailure", `endpoints "broker-filter" not found`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "broker-filter" not found`),
			},
			WantErr: true,
		}, {
			Name: "Successful Reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(testNS, true),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress)),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Successful Reconciliation, status update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(testNS, true),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "brokers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress)),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": inducing failure for update brokers`),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Successful Reconciliation, with single trigger",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(testNS, true),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}, {
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress)),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Fail Reconciliation, with single trigger, trigger status updated",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerBrokerFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: failed to find channelTemplate"),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}, {
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: failed to find channelTemplate")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "failed to find channelTemplate"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Broker being deleted, marks trigger as not ready due to broker missing",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerFinalizers("brokers.eventing.knative.dev"),
					WithBrokerResourceVersion(""),
					WithBrokerDeletionTimestamp),
				imcConfigMap(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "test-namespace/test-broker"`),
			},
		}, {
			Name: "Broker being deleted, marks trigger as not ready due to broker missing, fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerFinalizers("brokers.eventing.knative.dev"),
					WithBrokerResourceVersion(""),
					WithBrokerDeletionTimestamp),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				imcConfigMap(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "triggers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerUpdateStatusFailed", `Failed to update Trigger's status: inducing failure for update triggers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "Trigger reconcile failed: inducing failure for update triggers"),
			},
			WantErr: true,
		}, {
			Name: "Trigger being deleted",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerDeleted,
					WithTriggerSubscriberURI(subscriberURI))}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerDeleted,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "Trigger subscription create fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI))}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
			},
		}, {
			Name: "Trigger subscription create fails, update status fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI))}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
				InduceFailure("update", "triggers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerUpdateStatusFailed", "Failed to update Trigger's status: inducing failure for update triggers"),
			},
		}, {
			Name: "Trigger subscription delete fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for delete subscriptions"))},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Delete Trigger's subscription failed: inducing failure for delete subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for delete subscriptions"),
			},
		}, {
			Name: "Trigger subscription create after delete fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
			},
		}, {
			Name: "Trigger subscription not owned by Trigger",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeFilterSubscriptionNotOwnedByTrigger()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerUID(triggerUID),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", `trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", `Trigger reconcile failed: trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
			},
		}, {
			Name: "Trigger subscription update works",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady()),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "Trigger has subscriber ref exists",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists and URI",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists kubernetes Service",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberKubernetesServiceAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(k8sServiceResolvedURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", `Trigger reconcile failed: failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
				),
			}},
		}, {
			Name: "Subscription not ready, trigger marked not ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeFalseStatusSubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerNotSubscribed("testInducedError", "test induced error"),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Subscription ready, trigger marked ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Dependency doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: propagating dependency readiness: getting the dependency: pingsources.sources.knative.dev \"test-ping-source\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: pingsources.sources.knative.dev \"test-ping-source\" not found"),
				),
			}},
		}, {
			Name: "The status of Dependency is False",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeFalseStatusPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyFailed("NotFound", ""),
				),
			}},
		}, {
			Name: "The status of Dependency is Unknown",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeUnknownStatusCronJobSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("", ""),
				),
			}},
		},
		{
			Name: "Dependency generation not equal",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeGenerationNotEqualPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration))),
			}},
		},
		{
			Name: "Dependency ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeReadyPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		},

		{
			Name: "Trigger has deprecated named subscriber",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscriptionDeprecatedName(triggerNameLong, triggerUIDLong),
				makeReadyPingSource(),
				NewTrigger(triggerNameLong, testNS, brokerName,
					WithTriggerUID(triggerUIDLong),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, subscriptionDeleted, `Deprecated subscription removed: "%s/%s"`, testNS, makeReadySubscriptionDeprecatedName(triggerNameLong, triggerUIDLong).Name),
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerNameLong, testNS, brokerName,
					WithTriggerUID(triggerUIDLong),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeReadySubscriptionWithCustomData(triggerNameLong, triggerUIDLong),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: makeReadySubscriptionDeprecatedName(triggerNameLong, triggerUIDLong).Name,
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			kubeClientSet:      fakekubeclient.Get(ctx),
			subscriptionLister: listers.GetSubscriptionLister(),
			triggerLister:      listers.GetTriggerLister(),

			endpointsLister:    listers.GetEndpointsLister(),
			kresourceTracker:   duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1a1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		}
		return broker.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetBrokerLister(),
			controller.GetEventRecorder(ctx),
			r, "MTChannelBasedBroker")

	},
		false,
		logger,
	))
}

func config() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       configMapName,
		Namespace:  testNS,
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
}

func imcConfigMap() *corev1.ConfigMap {
	return NewConfigMap(configMapName, testNS,
		WithConfigMapData(map[string]string{"channelTemplateSpec": imcSpec}))
}

func createChannel(namespace string, ready bool) *unstructured.Unstructured {
	var labels map[string]interface{}
	var annotations map[string]interface{}
	var name string
	var url string
	name = fmt.Sprintf("%s-kne-trigger", brokerName)
	labels = map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations = map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
	}
	url = fmt.Sprintf("http://%s", triggerChannelHostname)
	if ready {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1",
				"kind":       "InMemoryChannel",
				"metadata": map[string]interface{}{
					"creationTimestamp": nil,
					"namespace":         namespace,
					"name":              name,
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "eventing.knative.dev/v1",
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               "Broker",
							"name":               brokerName,
							"uid":                "",
						},
					},
					"labels":      labels,
					"annotations": annotations,
				},
				"status": map[string]interface{}{
					"address": map[string]interface{}{
						"url": url,
					},
				},
			},
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "eventing.knative.dev/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Broker",
						"name":               brokerName,
						"uid":                "",
					},
				},
				"labels":      labels,
				"annotations": annotations,
			},
		},
	}
}

func createChannelNoHostInUrl(namespace string) *unstructured.Unstructured {
	name := fmt.Sprintf("%s-kne-trigger", brokerName)
	labels := map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations := map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "eventing.knative.dev/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Broker",
						"name":               brokerName,
						"uid":                "",
					},
				},
				"labels":      labels,
				"annotations": annotations,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": "http://",
				},
			},
		},
	}
}

func createTriggerChannelRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       fmt.Sprintf("%s-kne-trigger", brokerName),
	}
}

func makeServiceURI() *apis.URL {
	return &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("broker-filter.%s.svc.%s", systemNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
	}
}
func makeFilterSubscription() *messagingv1.Subscription {
	return resources.NewSubscription(makeTrigger(), createTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeEmptyDelivery())
}

func makeTrigger() *eventingv1.Trigger {
	return &eventingv1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: eventingv1.TriggerSpec{
			Broker: brokerName,
			Filter: &eventingv1.TriggerFilter{
				Attributes: map[string]string{"Source": "Any", "Type": "Any"},
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

func makeBrokerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
		Namespace:  testNS,
		Name:       brokerName,
	}
}
func makeEmptyDelivery() *eventingduckv1.DeliverySpec {
	return nil
}

func allBrokerObjectsReadyPlus(objs ...runtime.Object) []runtime.Object {
	brokerObjs := []runtime.Object{
		NewBroker(brokerName, testNS,
			WithBrokerClass(eventing.MTChannelBrokerClassValue),
			WithBrokerConfig(config()),
			WithInitBrokerConditions,
			WithBrokerReady,
			WithBrokerFinalizers("brokers.eventing.knative.dev"),
			WithBrokerResourceVersion(""),
			WithBrokerAddressURI(brokerAddress)),
		createChannel(testNS, true),
		imcConfigMap(),
		NewEndpoints(filterServiceName, systemNS,
			WithEndpointsLabels(FilterLabels()),
			WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
		NewEndpoints(ingressServiceName, systemNS,
			WithEndpointsLabels(IngressLabels()),
			WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
	}
	return append(brokerObjs[:], objs...)
}

// Just so we can test subscription updates
func makeDifferentReadySubscription() *messagingv1.Subscription {
	s := makeFilterSubscription()
	s.Spec.Subscriber.URI = apis.HTTP("different.example.com")
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeFilterSubscriptionNotOwnedByTrigger() *messagingv1.Subscription {
	sub := makeFilterSubscription()
	sub.OwnerReferences = []metav1.OwnerReference{}
	return sub
}

func makeReadySubscription() *messagingv1.Subscription {
	s := makeFilterSubscription()
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeReadySubscriptionDeprecatedName(triggerName, triggerUID string) *messagingv1.Subscription {
	s := makeFilterSubscription()
	t := NewTrigger(triggerName, testNS, brokerName)
	t.UID = types.UID(triggerUID)
	s.Name = utils.GenerateFixedName(t, fmt.Sprintf("%s-%s", brokerName, triggerName))
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeReadySubscriptionWithCustomData(triggerName, triggerUID string) *messagingv1.Subscription {
	t := makeTrigger()
	t.Name = triggerName
	t.UID = types.UID(triggerUID)

	uri := makeServiceURI()
	uri.Path = fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID)

	return resources.NewSubscription(t, createTriggerChannelRef(), makeBrokerRef(), uri, makeEmptyDelivery())
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

func makeFalseStatusSubscription() *messagingv1.Subscription {
	s := makeFilterSubscription()
	s.Status.MarkReferencesNotResolved("testInducedError", "test induced error")
	return s
}

func makeFalseStatusPingSource() *sourcesv1alpha2.PingSource {
	return rtv1alpha1.NewPingSourceV1Alpha2(pingSourceName, testNS, rtv1alpha1.WithPingSourceV1A2SinkNotFound)
}

func makeUnknownStatusCronJobSource() *sourcesv1alpha2.PingSource {
	cjs := rtv1alpha1.NewPingSourceV1Alpha2(pingSourceName, testNS)
	cjs.Status.InitializeConditions()
	return cjs
}

func makeGenerationNotEqualPingSource() *sourcesv1alpha2.PingSource {
	c := makeFalseStatusPingSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyPingSource() *sourcesv1alpha2.PingSource {
	u, _ := apis.ParseURL(sinkURI)
	return rtv1alpha1.NewPingSourceV1Alpha2(pingSourceName, testNS,
		rtv1alpha1.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			Schedule: testSchedule,
			JsonData: testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: brokerDestv1,
			},
		}),
		rtv1alpha1.WithInitPingSourceV1A2Conditions,
		rtv1alpha1.WithValidPingSourceV1A2Schedule,
		rtv1alpha1.WithValidPingSourceV1A2Resources,
		rtv1alpha1.WithPingSourceV1A2Deployed,
		rtv1alpha1.WithPingSourceV1A2CloudEventAttributes,
		rtv1alpha1.WithPingSourceV1A2Sink(u),
	)
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

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

// FilterLabels generates the labels present on all resources representing the filter of the given
// Broker.
func FilterLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "filter",
	}
}

func IngressLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "ingress",
	}
}
