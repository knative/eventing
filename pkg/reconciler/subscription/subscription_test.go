/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
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

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	subscriberName = "subscriber"
	replyName      = "reply"
	channelName    = "origin"
	serviceName    = "service"

	subscriptionUID        = subscriptionName + "-abc-123"
	subscriptionName       = "testsubscription"
	testNS                 = "testnamespace"
	subscriptionGeneration = 1
)

// subscriptions have: channel -> SUB -> subscriber -viaSub-> reply

var (
	channelDNS = "channel.mynamespace.svc." + utils.GetClusterDomainName()
	channelURI = apis.HTTP(channelDNS)

	subscriberDNS         = "subscriber.mynamespace.svc." + utils.GetClusterDomainName()
	subscriberURI         = apis.HTTP(subscriberDNS)
	subscriberURIWithPath = &apis.URL{Scheme: "http", Host: subscriberDNS, Path: "/"}

	replyDNS = "reply.mynamespace.svc." + utils.GetClusterDomainName()
	replyURI = apis.HTTP(replyDNS)

	serviceDNS         = serviceName + "." + testNS + ".svc." + utils.GetClusterDomainName()
	serviceURI         = apis.HTTP(serviceDNS)
	serviceURIWithPath = &apis.URL{Scheme: "http", Host: serviceDNS, Path: "/"}

	subscriberGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Subscriber",
	}

	nonAddressableGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Trigger",
	}

	nonSubscribableGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "EventType",
	}

	nonSubscribableCRDName = "eventtypes.eventing.knative.dev"

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}

	channelGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
	_ = apiextensionsv1beta1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
			//}, { // TODO: there is a bug in the controller, it will query for ""
			//	Name: "incomplete subscription",
			//	Objects: []runtime.Object{
			//		NewSubscription(subscriptionName, testNS),
			//	},
			//	Key:     "foo/incomplete",
			//	WantErr: true,
			//	WantEvents: []string{
			//		Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//	},
		}, {
			Name: "subscription, but channel does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. channels.messaging.knative.dev %q not found", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(channelReferenceFailed, fmt.Sprintf("Failed to get Spec.Channel as Channelable duck type. channels.messaging.knative.dev %q not found", channelName)),
				),
			}},
		}, {
			Name: "subscription, but channel crd does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewUnstructured(nonSubscribableGVK, channelName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to validate spec.channel: customresourcedefinition.apiextensions.k8s.io %q not found", nonSubscribableCRDName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(channelReferenceFailed, fmt.Sprintf("Failed to validate spec.channel: customresourcedefinition.apiextensions.k8s.io %q not found", nonSubscribableCRDName)),
				),
			}},
		}, {
			Name: "subscription, but channel crd does not contain subscribable label",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewUnstructured(nonSubscribableGVK, channelName, testNS),
				NewCustomResourceDefinition(nonSubscribableCRDName),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to validate spec.channel: crd %q does not contain mandatory label %q", nonSubscribableCRDName, channelLabelKey),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(channelReferenceFailed, fmt.Sprintf("Failed to validate spec.channel: crd %q does not contain mandatory label %q", nonSubscribableCRDName, channelLabelKey)),
				),
			}},
		}, {
			Name: "subscription, but channel crd contains invalid subscribable label value",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewUnstructured(nonSubscribableGVK, channelName, testNS),
				NewCustomResourceDefinition(nonSubscribableCRDName,
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: "whatever"})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to validate spec.channel: crd label %s has invalid value %q", channelLabelKey, "whatever"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(nonSubscribableGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(channelReferenceFailed, fmt.Sprintf("Failed to validate spec.channel: crd label %s has invalid value %q", channelLabelKey, "whatever")),
				),
			}},
		}, {
			Name: "subscription, but subscriber is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: address not set for &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, "Failed to resolve spec.subscriber: address not set for &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}"),
				),
			}},
		}, {
			Name: "subscription, but subscriber does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: failed to get ref &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}: subscribers.eventing.knative.dev %q not found", subscriberName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, fmt.Sprintf("Failed to resolve spec.subscriber: failed to get ref &ObjectReference{Kind:Subscriber,Namespace:testnamespace,Name:subscriber,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}: subscribers.eventing.knative.dev %q not found", subscriberName)),
				),
			}},
		}, {
			Name: "subscription, reply does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS)),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReplyResolveFailed", "Failed to resolve spec.reply: failed to get ref &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:reply,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}: channels.messaging.knative.dev %q not found", replyName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReplyDeprecated(),
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionReferencesNotResolved(replyResolveFailed, fmt.Sprintf("Failed to resolve spec.reply: failed to get ref &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:reply,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}: channels.messaging.knative.dev %q not found", replyName)),
				),
			}},
		}, {
			Name: "subscription, reply is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(nonAddressableGVK, replyName), // reply will be a nonAddressableGVK for this test
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
				NewUnstructured(nonAddressableGVK, replyName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, replyResolveFailed, "Failed to resolve spec.reply: address not set for &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:reply,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionReply(nonAddressableGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReplyDeprecated(),
					WithSubscriptionReferencesNotResolved(replyResolveFailed, "Failed to resolve spec.reply: address not set for &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:reply,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,}"),
				),
			}},
		}, {
			Name: "subscription, valid channel+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber(subscriptionUID),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,

					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReply(channelGVK, replyName),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber(subscriptionUID),
				),
				NewChannel(replyName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(replyDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReplyDeprecated(),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+reply - not deprecated",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReplyNotDeprecated(channelGVK, replyName),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber(subscriptionUID),
				),
				NewChannel(replyName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(replyDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReplyNotDeprecated(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+subscriber+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber(subscriptionUID),
				),
				NewChannel(replyName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(replyDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReplyDeprecated(),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid remove reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
					}),
					WithChannelReadySubscriberAndGeneration(subscriptionUID, subscriptionGeneration),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, Generation: subscriptionGeneration, SubscriberURI: subscriberURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid remove subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(channelGVK, channelName),
					WithInitSubscriptionConditions,
					WithSubscriptionReply(channelGVK, replyName),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
					}),
					WithChannelReadySubscriberAndGeneration(subscriptionUID, subscriptionGeneration),
				),
				NewChannel(replyName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(replyDNS),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReply(channelGVK, replyName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionReplyDeprecated(),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, Generation: subscriptionGeneration, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+subscriber as service",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber(subscriptionUID),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: serviceURIWithPath, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, two subscribers for a channel",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				// an already rec'ed subscription
				NewSubscription("b-"+subscriptionName, testNS,
					WithSubscriptionUID("b-"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
					WithChannelReadySubscriber("a-"+subscriptionUID),
					WithChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewCustomResourceDefinition("channels.messaging.knative.dev",
					WithCustomResourceDefinitionLabels(map[string]string{channelLabelKey: channelLabelValue})),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", "a-"+subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURIWithPath),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: "a-" + subscriptionUID, SubscriberURI: serviceURIWithPath, DeprecatedRef: &corev1.ObjectReference{Name: "a-" + subscriptionName, Namespace: testNS, UID: "a-" + subscriptionUID}},
					{UID: "b-" + subscriptionUID, SubscriberURI: serviceURIWithPath, DeprecatedRef: &corev1.ObjectReference{Name: "b-" + subscriptionName, Namespace: testNS, UID: "b-" + subscriptionUID}},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		}, {
			Name: "subscription deleted",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewChannel(replyName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					WithSubscriptionDeleted,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = addressable.WithDuck(ctx)
		return &Reconciler{
			Base:                           reconciler.NewBase(ctx, controllerAgentName, cmw),
			subscriptionLister:             listers.GetSubscriptionLister(),
			channelableTracker:             duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			destinationResolver:            resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			customResourceDefinitionLister: listers.GetCustomResourceDefinitionLister(),
		}
	}, false, logger))
}

func TestFinalizers(t *testing.T) {
	testCases := []struct {
		name     string
		original sets.String
		add      bool
		want     sets.String
	}{
		{
			name:     "empty, add",
			original: sets.NewString(),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "empty, delete",
			original: sets.NewString(),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, delete",
			original: sets.NewString(finalizerName),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, add",
			original: sets.NewString(finalizerName),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "existing two, delete",
			original: sets.NewString(finalizerName, "someother"),
			add:      false,
			want:     sets.NewString("someother"),
		}, {
			name:     "existing two, no change",
			original: sets.NewString(finalizerName, "someother"),
			add:      true,
			want:     sets.NewString(finalizerName, "someother"),
		},
	}

	for _, tc := range testCases {
		original := &v1alpha1.Subscription{}
		original.Finalizers = tc.original.List()
		if tc.add {
			addFinalizer(original)
		} else {
			removeFinalizer(original)
		}
		has := sets.NewString(original.Finalizers...)
		diff := has.Difference(tc.want)
		if diff.Len() > 0 {
			t.Errorf("%q failed, diff: %+v", tc.name, diff)
		}
	}
}

func addFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Insert(finalizerName)
	sub.Finalizers = finalizers.List()
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
		spec = fmt.Sprintf(`{"subscribable":{"subscribers":%s}}`, subs)
	} else {
		spec = `{"subscribable":{}}`
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
