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
	"encoding/json"
	"fmt"
	testing2 "github.com/knative/eventing/pkg/reconciler/testing"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
)

const (
	subscriberName = "subscriber"
	replyName      = "reply"
	channelName    = "origin"
	serviceName    = "service"

	subscriptionUID  = subscriptionName + "-abc-123"
	subscriptionName = "testsubscription"
	testNS           = "testnamespace"
)

// subscriptions have: channel -> SUB -> subscriber -viaSub-> reply

var (
	channelDNS = "channel.mynamespace.svc." + utils.GetClusterDomainName()
	channelURI = "http://" + channelDNS + "/"

	subscriberDNS = "subscriber.mynamespace.svc." + utils.GetClusterDomainName()
	subscriberURI = "http://" + subscriberDNS + "/"

	replyDNS = "reply.mynamespace.svc." + utils.GetClusterDomainName()
	replyURI = "http://" + replyDNS + "/"

	serviceDNS = serviceName + "." + testNS + ".svc." + utils.GetClusterDomainName()
	serviceURI = "http://" + serviceDNS + "/"

	subscriberGVK = metav1.GroupVersionKind{
		Group:   "testing.eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Subscriber",
	}

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}

	channelGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
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
			Name: "subscription, but subscriber is not addressable",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: status does not contain address"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
				),
			}},
		}, {
			Name: "subscription, but subscriber does not exist",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: subscribers.testing.eventing.knative.dev %q not found", subscriberName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
				),
			}},
		}, {
			Name: "subscription, reply does not exist",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS)),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ResultResolveFailed", "Failed to resolve spec.reply: channels.eventing.knative.dev %q not found", replyName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
		}, {
			Name: "subscription, reply is not addressable",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(subscriberGVK, replyName), // reply will be a subscriberGVK for this test
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewUnstructured(subscriberGVK, replyName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ResultResolveFailed", "Failed to resolve spec.reply: status does not contain address"),
				Eventf(corev1.EventTypeWarning, "SubscriptionUpdateStatusFailed", "Failed to update Subscription's status: status does not contain address"), // TODO: BUGBUG THIS IS WEIRD
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(subscriberGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
		}, {
			Name: "subscription, valid channel+subscriber",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+reply",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewChannel(replyName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid channel+subscriber+reply",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewChannel(replyName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					testing2.WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid remove reply",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					testing2.WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
					testing2.WithChannelSubscribers([]v1alpha1.ChannelSubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
					}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, valid remove subscriber",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithInitSubscriptionConditions,
					testing2.WithSubscriptionReply(channelGVK, replyName),
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					testing2.WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
					testing2.WithChannelSubscribers([]v1alpha1.ChannelSubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
					}),
				),
				testing2.NewChannel(replyName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionReply(replyURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, channel+subscriber as service, does not exist",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: services %q not found", serviceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
				),
			}},
		}, {
			Name: "subscription, valid channel+subscriber as service",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: serviceURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription, two subscribers for a channel",
			Objects: []runtime.Object{
				testing2.NewSubscription("a_"+subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				// an already rec'ed subscription
				testing2.NewSubscription("b_"+subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a_" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", "a_"+subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription("a_"+subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
					{UID: "b_" + subscriptionUID, SubscriberURI: serviceURI, DeprecatedRef: &corev1.ObjectReference{Name: "b_" + subscriptionName, Namespace: testNS, UID: "b_" + subscriptionUID}},
					{UID: "a_" + subscriptionUID, SubscriberURI: serviceURI, DeprecatedRef: &corev1.ObjectReference{Name: "a_" + subscriptionName, Namespace: testNS, UID: "a_" + subscriptionUID}},
				}),
				patchFinalizers(testNS, "a_"+subscriptionName),
			},
		}, {
			Name: "subscription deleted",
			Objects: []runtime.Object{
				testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionFinalizers(finalizerName),
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					testing2.WithSubscriptionDeleted,
				),
				testing2.NewUnstructured(subscriberGVK, subscriberName, testNS,
					testing2.WithUnstructuredAddressable(subscriberDNS),
				),
				testing2.NewChannel(channelName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(channelDNS),
				),
				testing2.NewChannel(replyName, testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelAddress(replyDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", subscriptionName),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewSubscription(subscriptionName, testNS,
					testing2.WithSubscriptionChannel(channelGVK, channelName),
					testing2.WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					testing2.WithSubscriptionReply(channelGVK, replyName),
					testing2.WithInitSubscriptionConditions,
					testing2.MarkSubscriptionReady,
					testing2.WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
					testing2.WithSubscriptionDeleted,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, testing2.MakeFactory(func(listers *testing2.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}))

}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)

	subscriptionInformer := eventingInformer.Eventing().V1alpha1().Subscriptions()
	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	}, subscriptionInformer)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
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
		original := &eventingv1alpha1.Subscription{}
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

func addFinalizer(sub *eventingv1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Insert(finalizerName)
	sub.Finalizers = finalizers.List()
}

func patchSubscribers(namespace, name string, subscribers []v1alpha1.ChannelSubscriberSpec) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	var spec string
	if subscribers != nil {
		b, err := json.Marshal(subscribers)
		ss := make([]map[string]interface{}, 0)
		err = json.Unmarshal(b, &ss)
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
