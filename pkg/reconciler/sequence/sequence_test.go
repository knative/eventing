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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/flows/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"

	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1beta1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/flows/v1beta1/sequence"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	rt "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS             = "test-namespace"
	sequenceName       = "test-sequence"
	replyChannelName   = "reply-channel"
	sequenceGeneration = 7
)

func init() {
	// Add types to scheme
	_ = v1beta1.AddToScheme(scheme.Scheme)
	_ = duckv1beta1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func createReplyChannel(channelName string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "messaging.knative.dev/v1beta1",
			Kind:       "InMemoryChannel",
			Name:       channelName,
			Namespace:  testNS,
		},
	}
}

func createChannel(sequenceName string, stepNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1beta1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.SequenceChannelName(sequenceName, stepNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1beta1",
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

func createDestination(stepNumber int) duckv1.Destination {
	uri := apis.HTTP("example.com")
	uri.Path = fmt.Sprintf("%d", stepNumber)
	return duckv1.Destination{
		URI: uri,
	}
}

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + sequenceName
	imc := &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1beta1",
			Kind:       "InMemoryChannel",
		},
		Spec: &runtime.RawExtension{Raw: []byte("{}")},
	}

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "deleting",
		Key:  pKey,
		Objects: []runtime.Object{
			rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceDeleted)},
		WantErr: false,
	}, {
		Name: "singlestep",
		Key:  pKey,
		Objects: []runtime.Object{
			rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "SequenceReconciled", `Sequence reconciled: "test-namespace/test-sequence"`),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}),
				rt.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				rt.WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				rt.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				rt.WithSequenceChannelStatuses([]v1beta1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
				rt.WithSequenceSubscriptionStatuses([]v1beta1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
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
			rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "SequenceReconciled", `Sequence reconciled: "test-namespace/test-sequence"`),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0, rt.NewSequence(sequenceName, testNS,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				rt.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				rt.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				rt.WithSequenceChannelStatuses([]v1beta1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
				rt.WithSequenceSubscriptionStatuses([]v1beta1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
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
			rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceGeneration(sequenceGeneration),
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))},
		WantErr: false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "SequenceReconciled", `Sequence reconciled: "test-namespace/test-sequence"`),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0,
				rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}))),
			resources.NewSubscription(1,
				rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),
			resources.NewSubscription(2,
				rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}})))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceGeneration(sequenceGeneration),
				rt.WithSequenceStatusObservedGeneration(sequenceGeneration),
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}),
				rt.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				rt.WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				rt.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				rt.WithSequenceChannelStatuses([]v1beta1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
				rt.WithSequenceSubscriptionStatuses([]v1beta1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
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
			rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))},
		WantErr: false,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "SequenceReconciled", `Sequence reconciled: "test-namespace/test-sequence"`),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0, rt.NewSequence(sequenceName, testNS,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))),
			resources.NewSubscription(1, rt.NewSequence(sequenceName, testNS,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))),
			resources.NewSubscription(2, rt.NewSequence(sequenceName, testNS,
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}})))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rt.NewSequence(sequenceName, testNS,
				rt.WithInitSequenceConditions,
				rt.WithSequenceReply(createReplyChannel(replyChannelName)),
				rt.WithSequenceChannelTemplateSpec(imc),
				rt.WithSequenceSteps([]v1beta1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}),
				rt.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				rt.WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				rt.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				rt.WithSequenceChannelStatuses([]v1beta1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "InMemoryChannel",
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
				rt.WithSequenceSubscriptionStatuses([]v1beta1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1beta1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 2),
							Namespace:  testNS,
						},
					},
				})),
		}},
	},
		{
			Name: "sequenceupdatesubscription",
			Key:  pKey,
			Objects: []runtime.Object{
				rt.NewSequence(sequenceName, testNS,
					rt.WithInitSequenceConditions,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(1)}})),
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(0)}}))),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SequenceReconciled", `Sequence reconciled: "test-namespace/test-sequence"`),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{Name: resources.SequenceChannelName(sequenceName, 0)},
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(0, rt.NewSequence(sequenceName, testNS,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(1)}}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rt.NewSequence(sequenceName, testNS,
					rt.WithInitSequenceConditions,
					rt.WithSequenceChannelTemplateSpec(imc),
					rt.WithSequenceSteps([]v1beta1.SequenceStep{{Destination: createDestination(1)}}),
					rt.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					rt.WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					rt.WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					rt.WithSequenceChannelStatuses([]v1beta1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1beta1",
								Kind:       "InMemoryChannel",
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
					rt.WithSequenceSubscriptionStatuses([]v1beta1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1beta1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
					})),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		r := &Reconciler{
			sequenceLister:     listers.GetSequenceLister(),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			subscriptionLister: listers.GetSubscriptionLister(),
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
		}
		return sequence.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetSequenceLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}
