/*
Copyright 2021 The Knative Authors

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

package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/utils/pointer"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/network"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/subscription"
	"knative.dev/eventing/pkg/duck"

	. "knative.dev/pkg/reconciler/testing"

	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel/fake"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
)

const (
	subscriberName = "subscriber"
	replyName      = "reply"
	channelName    = "origin"
	serviceName    = "service"
	dlcName        = "dlc"
	dlc2Name       = "dlc2"

	subscriptionUID        = subscriptionName + "-abc-123"
	subscriptionName       = "testsubscription"
	testNS                 = "testnamespace"
	subscriptionGeneration = 1

	finalizerName = "subscriptions.messaging.knative.dev"
)

// subscriptions have: channel -> SUB -> subscriber -viaSub-> reply

var (
	channelDNS = "channel.mynamespace.svc." + network.GetClusterDomainName()

	subscriberDNS = "subscriber.mynamespace.svc." + network.GetClusterDomainName()
	subscriberURI = apis.HTTP(subscriberDNS)

	replyDNS = "reply.mynamespace.svc." + network.GetClusterDomainName()
	replyURI = apis.HTTP(replyDNS)

	serviceDNS         = serviceName + "." + testNS + ".svc." + network.GetClusterDomainName()
	serviceURI         = apis.HTTP(serviceDNS)
	serviceURIWithPath = &apis.URL{Scheme: "http", Host: serviceDNS, Path: "/"}

	dlcDNS = "dlc.mynamespace.svc." + network.GetClusterDomainName()
	dlcURI = apis.HTTP(dlcDNS)

	dlc2DNS = "dlc2.mynamespace.svc." + network.GetClusterDomainName()

	subscriberGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Subscriber",
	}

	nonAddressableGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1",
		Kind:    "Trigger",
	}

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}

	imcV1GVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "InMemoryChannel",
	}

	channelV1GVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Channel",
	}

	imcV1KRef = duckv1.KReference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       channelName,
	}
)

func TestAllCases(t *testing.T) {
	linear := eventingduck.BackoffPolicyLinear

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
			Name: "subscription goes ready",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(replyDNS),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
					// - Status Update -
					MarkSubscriptionReady,
				),
			}},
		}, {
			Name: "channel does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesResolvedUnknown(channelReferenceFailed, fmt.Sprintf("Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "channel does not exist - fail status update",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, subscriptionUpdateStatusFailed, "Failed to update status for %q: inducing failure for update subscriptions", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesResolvedUnknown(channelReferenceFailed, fmt.Sprintf("Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscriber is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: address not set for &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:messaging.knative.dev/v1,ResourceVersion:,FieldPath:,}"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, "Failed to resolve spec.subscriber: address not set for &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:messaging.knative.dev/v1,ResourceVersion:,FieldPath:,}"),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscriber does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: subscribers.messaging.knative.dev %q not found", subscriberName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, `Failed to resolve spec.subscriber: subscribers.messaging.knative.dev "subscriber" not found`),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "reply does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS)),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "ReplyResolveFailed", `Failed to resolve spec.reply: inmemorychannels.messaging.knative.dev %q not found`, replyName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionReferencesNotResolved(replyResolveFailed, fmt.Sprintf(`Failed to resolve spec.reply: inmemorychannels.messaging.knative.dev %q not found`, replyName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "reply is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(nonAddressableGVK, replyName, testNS), // reply will be a nonAddressableGVK for this test
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
				NewUnstructured(nonAddressableGVK, replyName, testNS),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, replyResolveFailed, "Failed to resolve spec.reply: address not set for &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:reply,UID:,APIVersion:eventing.knative.dev/v1,ResourceVersion:,FieldPath:,}"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionReply(nonAddressableGVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(replyResolveFailed, "Failed to resolve spec.reply: address not set for &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:reply,UID:,APIVersion:eventing.knative.dev/v1,ResourceVersion:,FieldPath:,}"),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc, valid channel+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,

					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc, valid channel+subscriber+missing delivery",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "DeadLetterSinkResolveFailed", `Failed to resolve spec.delivery.deadLetterSink: subscribers.messaging.knative.dev "dlc" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved("DeadLetterSinkResolveFailed", `Failed to resolve spec.delivery.deadLetterSink: subscribers.messaging.knative.dev "dlc" not found`),
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1beta imc, valid channel+subscriber+delivery",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlcDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, Delivery: &eventingduck.DeliverySpec{DeadLetterSink: &duckv1.Destination{URI: apis.HTTP("dlc.mynamespace.svc.cluster.local")}}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 channel+v1 imc backing channel+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithBackingChannelObjRef(&imcV1KRef),
					WithBackingChannelReady,
					WithChannelAddress("example.com"),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
					WithInMemoryChannelReady("example.com"),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,

					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 channel+backing v1 imc channel not ready+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithBackingChannelObjRef(&imcV1KRef),
					WithBackingChannelReady,
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReadySubscriber(subscriptionUID),
					WithInMemoryChannelReady("example.com"),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "ChannelReferenceFailed", "Failed to get Spec.Channel or backing channel: channel is not ready."),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesResolvedUnknown("ChannelReferenceFailed", "Failed to get Spec.Channel or backing channel: channel is not ready."),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc channel+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+reply - not deprecated",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+subscriber+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+valid remove reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithInMemoryChannelReadySubscriberAndGeneration(subscriptionUID, subscriptionGeneration),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionStatusObservedGeneration(subscriptionGeneration),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, Generation: subscriptionGeneration, SubscriberURI: subscriberURI},
				}),
			},
		}, {
			Name: "v1 imc+remove subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithInitSubscriptionConditions,
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithInMemoryChannelReadySubscriberAndGeneration(subscriptionUID, subscriptionGeneration),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
					WithSubscriptionStatusObservedGeneration(subscriptionGeneration),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, Generation: subscriptionGeneration, ReplyURI: replyURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+subscriber as service",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: serviceURIWithPath},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+two subscribers for a channel",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				// an already rec'ed subscription
				NewSubscription("b-"+subscriptionName, testNS,
					WithSubscriptionUID("b-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: "a-" + subscriptionUID, SubscriberURI: serviceURIWithPath},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		}, {
			Name: "v1 imc+two subscribers for a channel - update delivery",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlcDNS),
				),
				// an already rec'ed subscription
				NewSubscription("b-"+subscriptionName, testNS,
					WithSubscriptionUID("b-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: "a-" + subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlcName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: "a-" + subscriptionUID, SubscriberURI: serviceURIWithPath, Delivery: &eventingduck.DeliverySpec{DeadLetterSink: &duckv1.Destination{URI: apis.HTTP("dlc.mynamespace.svc.cluster.local")}}},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc+two subscribers for a channel - update delivery - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT1S"),
					}),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlcDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT1S"),
					}),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURIWithPath,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dlc.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32Ptr(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.StringPtr("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - delivery defaulting - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlcDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT1S"),
					}),
				),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURIWithPath,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dlc.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32Ptr(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.StringPtr("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - don't default delivery - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT1S"),
					}),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlcDNS),
				),
				NewUnstructured(subscriberGVK, dlc2Name, testNS,
					WithUnstructuredAddressable(dlc2DNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlc2Name,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(20),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT10S"),
					}),
				),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32Ptr(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("PT1S"),
					}),
					WithSubscriptionDeadLetterSinkURI(dlcURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURIWithPath,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dlc.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32Ptr(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.StringPtr("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc+deleted - channel patch succeeded",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI},
					}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberRemoved", "Subscription was removed from channel \"origin\""),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
				patchRemoveFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription not deleted - channel patch fails",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI},
					}),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("patch", "inmemorychannels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "PhysicalChannelSyncFailed", fmt.Sprintf("Failed to synchronize to channel %q: %s", channelName, "inducing failure for patch inmemorychannels")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					MarkNotAddedToChannel("PhysicalChannelSyncFailed", "Failed to sync physical Channel: inducing failure for patch inmemorychannels"),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
			},
		}, {
			Name: "subscription deleted - channel does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, subscriptionName),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			dynamicClientSet:    dynamicclient.Get(ctx),
			subscriptionLister:  listers.GetSubscriptionLister(),
			channelLister:       listers.GetMessagingChannelLister(),
			channelableTracker:  duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			destinationResolver: resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			tracker:             &FakeTracker{},
		}
		return subscription.NewReconciler(ctx, logger,
			eventingclient.Get(ctx), listers.GetSubscriptionLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func WithSubscriptionDeliverySpec(d *eventingduck.DeliverySpec) SubscriptionOption {
	return func(v *messagingv1.Subscription) {
		v.Spec.Delivery = d
	}
}

func patchSubscribers(namespace, name string, subscribers []eventingduck.SubscriberSpec) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	var spec string
	if subscribers != nil {
		b, err := json.Marshal(subscribers)
		if err != nil {
			return action
		}
		ss := make([]map[string]interface{}, 0)
		err = json.Unmarshal(b, &ss)
		if err != nil {
			return action
		}
		subs, err := json.Marshal(ss)
		if err != nil {
			return action
		}
		spec = fmt.Sprintf(`{"subscribers":%s}`, subs)
	} else {
		spec = `{"subscribers":null}`
	}

	patch := `{"spec":` + spec + `}`
	action.Patch = []byte(patch)
	return action
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
