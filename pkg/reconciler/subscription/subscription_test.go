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
	"testing"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	. "github.com/knative/eventing/pkg/reconciler/testing"
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
		Group:   "eventing.knative.dev",
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
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: status does not contain address"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
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
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: subscribers.eventing.knative.dev %q not found", subscriberName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
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
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ResultResolveFailed", "Failed to resolve spec.reply: channels.eventing.knative.dev %q not found", replyName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
				),
			}},
		}, {
			Name: "subscription, reply is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(subscriberGVK, replyName), // reply will be a subscriberGVK for this test
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriberDNS),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewUnstructured(subscriberGVK, replyName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionUpdateStatusFailed", "Failed to update Subscription's status: invalid value: Subscriber: spec.reply.kind\nonly 'Channel' kind is allowed"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithSubscriptionReply(subscriberGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
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
				),
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
				patchSubscribers(testNS, channelName, []v1alpha1.ChannelSubscriberSpec{
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
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReply(channelGVK, replyName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
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
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
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
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
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
					WithChannelSubscribers([]v1alpha1.ChannelSubscriberSpec{
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
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(subscriberURI),
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
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
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
					WithChannelSubscribers([]v1alpha1.ChannelSubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI, DeprecatedRef: &corev1.ObjectReference{Name: subscriptionName, Namespace: testNS, UID: subscriptionUID}},
					}),
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
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionReply(channelGVK, replyName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionReply(replyURI),
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
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: services %q not found", serviceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
				),
			}},
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
				),
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
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
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
				NewSubscription("a_"+subscriptionName, testNS,
					WithSubscriptionUID("a_"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
				),
				// an already rec'ed subscription
				NewSubscription("b_"+subscriptionName, testNS,
					WithSubscriptionUID("b_"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(channelDNS),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a_" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: %q", "a_"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriptionReadinessChanged", "Subscription %q became ready", "a_"+subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a_"+subscriptionName, testNS,
					WithSubscriptionUID("a_"+subscriptionUID),
					WithSubscriptionChannel(channelGVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(serviceURI),
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

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(opt, controllerAgentName),
			subscriptionLister:  listers.GetSubscriptionLister(),
			tracker:             tracker.New(func(string) {}, 0),
			addressableInformer: &fakeAddressableInformer{},
		}
	},
		false,
	))
}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)

	subscriptionInformer := eventingInformer.Eventing().V1alpha1().Subscriptions()
	channelInformer := eventingInformer.Eventing().V1alpha1().Channels()
	addressableInformer := &fakeAddressableInformer{}
	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	}, subscriptionInformer, channelInformer, addressableInformer)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

type fakeAddressableInformer struct{}

func (*fakeAddressableInformer) TrackInNamespace(tracker.Interface, metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error { return nil }
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
