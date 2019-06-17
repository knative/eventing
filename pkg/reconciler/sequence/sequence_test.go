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

package sequence

import (
	"context"
	"fmt"
	"testing"

	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/sequence/resources"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
)

const (
	testNS           = "test-namespace"
	sequenceName     = "test-sequence"
	sequenceUID      = "test-sequence-uid"
	replyChannelName = "reply-channel"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

type fakeAddressableInformer struct{}

func (*fakeAddressableInformer) NewTracker(callback func(string), lease time.Duration) duck.AddressableTracker {
	return fakeAddressableTracker{}
}

type fakeAddressableTracker struct{}

func (fakeAddressableTracker) TrackInNamespace(metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error { return nil }
}

func (fakeAddressableTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	return nil
}

func (fakeAddressableTracker) OnChanged(obj interface{}) {
}

func createReplyChannel(channelName string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "inmemorychannel",
		Name:       channelName,
	}

}

func createChannel(sequenceName string, stepNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.SequenceChannelName(sequenceName, stepNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Sequence",
						"name":               sequenceName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}

}

func createSubscriber(stepNumber int) eventingv1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/%d", stepNumber)
	return eventingv1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + sequenceName
	imc := v1alpha1.ChannelTemplateSpec{
		metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
		},
		runtime.RawExtension{Raw: []byte("{}")},
	}

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
			Name: "deleting",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
		}, {
			Name: "singlestep",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}),
					reconciletesting.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithSequenceChannelStatuses([]v1alpha1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
					}),
					reconciletesting.WithSequenceSubscriptionStatuses([]v1alpha1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
					})),
			}},
		}, {
			Name: "singlestepwithreply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithSequenceChannelStatuses([]v1alpha1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
					}),
					reconciletesting.WithSequenceSubscriptionStatuses([]v1alpha1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
					})),
			}},
		}, {
			Name: "threestep",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(1, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(2, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2),
					}),
					reconciletesting.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithSequenceChannelStatuses([]v1alpha1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
					}),
					reconciletesting.WithSequenceSubscriptionStatuses([]v1alpha1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
						},
					})),
			}},
		}, {
			Name: "threestepwithreply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(1, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(2, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2),
					}),
					reconciletesting.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithSequenceChannelStatuses([]v1alpha1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionFalse,
								Reason:  "NotAddressable",
								Message: "Channel is not addressable",
							},
						},
					}),
					reconciletesting.WithSequenceSubscriptionStatuses([]v1alpha1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
						},
					})),
			}},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			sequenceLister:     listers.GetSequenceLister(),
			addressableTracker: fakeAddressableTracker{},
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}, false))
}
