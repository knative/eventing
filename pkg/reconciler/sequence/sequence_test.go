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

	"knative.dev/eventing/pkg/apis/feature"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/sequence"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"

	. "knative.dev/pkg/reconciler/testing"

	. "knative.dev/eventing/pkg/reconciler/testing/v1"
)

const (
	testNS                 = "test-namespace"
	sequenceName           = "test-sequence"
	replyChannelName       = "reply-channel"
	sequenceGeneration     = 7
	readyEventPolicyName   = "test-event-policy-ready"
	unreadyEventPolicyName = "test-event-policy-unready"
)

var (
	subscriberGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Subscription",
	}

	sequenceGVK = metav1.GroupVersionKind{
		Group:   "flows.knative.dev",
		Version: "v1",
		Kind:    "Sequence",
	}

	channelV1GVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "InMemoryChannel",
	}
)

func createReplyChannel(channelName string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
			Name:       channelName,
			Namespace:  testNS,
		},
	}
}

func createChannel(sequenceName string, stepNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.SequenceChannelName(sequenceName, stepNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1",
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

func apiVersion(gvk metav1.GroupVersionKind) string {
	groupVersion := gvk.Version
	if gvk.Group != "" {
		groupVersion = gvk.Group + "/" + gvk.Version
	}
	return groupVersion
}

func createDelivery(gvk metav1.GroupVersionKind, name, namespace string) *eventingduckv1.DeliverySpec {
	return &eventingduckv1.DeliverySpec{
		DeadLetterSink: &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
		},
	}
}

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + sequenceName
	imc := &messagingv1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
		},
		Spec: &runtime.RawExtension{Raw: []byte("{}")},
	}

	differentIMC := &messagingv1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "OtherKindOfChannel",
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
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceDeleted)},
		WantErr: false,
	}, {
		Name: "singlestep",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "singlestep-channelcreatefails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "inmemorychannels"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile channel test-sequence-kn-sequence-0 at step 0: failed to create channel {InMemoryChannel test-namespace test-sequence-kn-sequence-0  messaging.knative.dev/v1  }: inducing failure for create inmemorychannels"),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "failed to reconcile channel test-sequence-kn-sequence-0 at step 0: failed to create channel {InMemoryChannel test-namespace test-sequence-kn-sequence-0  messaging.knative.dev/v1  }: inducing failure for create inmemorychannels")),
		}},
	}, {
		Name: "singlestep-subscriptioncreatefails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "subscriptions"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile subscription resource for step: 0 : inducing failure for create subscriptions"),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "failed to reconcile subscription resource for step: 0 : inducing failure for create subscriptions"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "singlestep-subscriptiondeletefails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(differentIMC),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})))},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("delete", "subscriptions"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile subscription resource for step: 0 : inducing failure for delete subscriptions"),
		},
		WantCreates: []runtime.Object{createChannel(sequenceName, 0)},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Name: resources.SequenceChannelName(sequenceName, 0),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "failed to reconcile subscription resource for step: 0 : inducing failure for delete subscriptions"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "singlestep-subscriptioncreateafterdeletefails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(differentIMC),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})))},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "subscriptions"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile subscription resource for step: 0 : inducing failure for create subscriptions"),
		},
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Name: resources.SequenceChannelName(sequenceName, 0),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "failed to reconcile subscription resource for step: 0 : inducing failure for create subscriptions"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "singlestepwithreply",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "threestep",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceGeneration(sequenceGeneration),
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}))),
			resources.NewSubscription(1,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),
			resources.NewSubscription(2,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}})))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceGeneration(sequenceGeneration),
				WithSequenceStatusObservedGeneration(sequenceGeneration),
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "threestepwithdeliveryontwo",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceGeneration(sequenceGeneration),
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1), Delivery: createDelivery(subscriberGVK, "dlc1", testNS)},
					{Destination: createDestination(2), Delivery: createDelivery(subscriberGVK, "dlc2", testNS)}}))},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}))),
			resources.NewSubscription(1,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1), Delivery: createDelivery(subscriberGVK, "dlc1", testNS)}, {Destination: createDestination(2)}}))),
			resources.NewSubscription(2,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2), Delivery: createDelivery(subscriberGVK, "dlc2", testNS)}})))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceGeneration(sequenceGeneration),
				WithSequenceStatusObservedGeneration(sequenceGeneration),
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1), Delivery: createDelivery(subscriberGVK, "dlc1", testNS)},
					{Destination: createDestination(2), Delivery: createDelivery(subscriberGVK, "dlc2", testNS)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "threestepwithreply",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))),
			resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}))),
			resources.NewSubscription(2, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}})))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceReply(createReplyChannel(replyChannelName)),
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 2),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "sequenceupdatesubscription",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(1)}})),
			createChannel(sequenceName, 0),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantErr: false,
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Object: resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(1)}}))),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(1)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "sequenceupdate-remove-step",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}})),
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(2, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
		},
		WantErr: false,
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: resources.SequenceChannelName(sequenceName, 2),
			}, {
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
				},
				Name: resources.SequenceChannelName(sequenceName, 2),
			},
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}},
				))),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "sequenceupdate-remove-step-subscription-removal-fails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}})),
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(2, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
		},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("delete", "subscriptions"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for delete subscriptions"),
		},

		WantDeletes: []clientgotesting.DeleteActionImpl{
			{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: resources.SequenceChannelName(sequenceName, 2),
			}, {
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
				},
				Name: resources.SequenceChannelName(sequenceName, 2),
			},
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}},
				))),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceEventPoliciesReadyBecauseOIDCDisabled(),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "sequenceupdate-remove-step-channel-removal-fails",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}})),
			createChannel(sequenceName, 0),
			createChannel(sequenceName, 1),
			createChannel(sequenceName, 2),
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
			resources.NewSubscription(2, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)},
					{Destination: createDestination(2)},
				}))),
		},
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("delete", "inmemorychannels"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for delete inmemorychannels"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}},
				))),
		}},

		WantDeletes: []clientgotesting.DeleteActionImpl{
			{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
				},
				Name: resources.SequenceChannelName(sequenceName, 2),
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{
					{Destination: createDestination(0)},
					{Destination: createDestination(1)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 1),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				})),
		}},
	}, {
		Name: "Should provision applying EventPolicies",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			NewEventPolicy(readyEventPolicyName, testNS,
				WithReadyEventPolicyCondition,
				WithEventPolicyToRef(sequenceGVK, sequenceName),
			),
		},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				}),
				WithSequenceEventPoliciesReady(),
				WithSequenceEventPoliciesListed(readyEventPolicyName),
			),
		}},
	}, {
		Name: "Should mark as NotReady on unready EventPolicies",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			NewEventPolicy(unreadyEventPolicyName, testNS,
				WithUnreadyEventPolicyCondition("", ""),
				WithEventPolicyToRef(sequenceGVK, sequenceName),
			),
		},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				}),
				WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s are not ready", unreadyEventPolicyName))),
		}},
	}, {
		Name: "should list only Ready EventPolicies",
		Key:  pKey,
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			NewEventPolicy(unreadyEventPolicyName, testNS,
				WithUnreadyEventPolicyCondition("", ""),
				WithEventPolicyToRef(sequenceGVK, sequenceName),
			),
			NewEventPolicy(readyEventPolicyName, testNS,
				WithReadyEventPolicyCondition,
				WithEventPolicyToRef(sequenceGVK, sequenceName),
			),
		},
		WantErr: false,
		WantCreates: []runtime.Object{
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}}),
				WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
				WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
				WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
				WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
					{
						Channel: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "InMemoryChannel",
							Name:       resources.SequenceChannelName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Channel does not have Ready condition",
						},
					},
				}),
				WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
					{
						Subscription: corev1.ObjectReference{
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Subscription",
							Name:       resources.SequenceSubscriptionName(sequenceName, 0),
							Namespace:  testNS,
						},
						ReadyCondition: apis.Condition{
							Type:    apis.ConditionReady,
							Status:  corev1.ConditionUnknown,
							Reason:  "NoReady",
							Message: "Subscription does not have Ready condition",
						},
					},
				}),
				WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s are not ready", unreadyEventPolicyName)),
				WithSequenceEventPoliciesListed(readyEventPolicyName),
			),
		}},
	},

		{
			Name: "threestep with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}))},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)}}))),
				resources.NewSubscription(1,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),
				resources.NewSubscription(2,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
				// make the eventpolicy for the sequence

			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},

		{
			Name: "threestep with AuthZ enabled, and sequence has event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}})),
				makeSequenceEventPolicy(sequenceName),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)}}))),
				resources.NewSubscription(1,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),
				resources.NewSubscription(2,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}, {Destination: createDestination(2)}}))),

				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", "event policies test-sequence are not ready"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),

					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},

		{
			Name: "2 steps with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}}))},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)}}))),
				resources.NewSubscription(1,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}}))),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				// make the eventpolicy for the sequence

			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "2 steps with AuthZ enabled, and sequence has event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}})),
				makeSequenceEventPolicy(sequenceName),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)}}))),
				resources.NewSubscription(1,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}, {Destination: createDestination(1)}}))),

				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),

				// make the eventpolicy for the sequence

			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", "event policies test-sequence are not ready"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),

					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},

		{
			Name: "1 steps with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}}))},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "1 steps with AuthZ enabled, and sequence has event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}})),
				makeSequenceEventPolicy(sequenceName),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)}}))),

				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceGeneration(sequenceGeneration),
					WithSequenceStatusObservedGeneration(sequenceGeneration),
					WithSequenceEventPoliciesReady(),
					WithSequenceEventPoliciesListed(),
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}}),

					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", "event policies test-sequence are not ready"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),

					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 3 steps with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				resources.NewSubscription(2, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				// Making the event policy for the sequence:
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Name: resources.SequenceChannelName(sequenceName, 2),
				}, {
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
					},
					Name: resources.SequenceChannelName(sequenceName, 2),
				},
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceChannelName(sequenceName, 2)),
				},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}},
					))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 2 steps with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				// Making the event policy for the sequence:
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Name: resources.SequenceChannelName(sequenceName, 1),
				}, {
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
					},
					Name: resources.SequenceChannelName(sequenceName, 1),
				},
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceChannelName(sequenceName, 1)),
				},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Object: resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}},
					))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 1 steps with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{})),
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
					}))),
			},
			WantErr: true,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", "Failed to update status for \"test-sequence\": missing field(s): spec.steps"),
			},

			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 3 steps with AuthZ enabled, and sequence does have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}})),
				makeSequenceEventPolicy(sequenceName),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				resources.NewSubscription(2, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}))),
				// Making the event policy for the sequence:
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),

				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Name: resources.SequenceChannelName(sequenceName, 2),
				}, {
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
					},
					Name: resources.SequenceChannelName(sequenceName, 2),
				},
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceChannelName(sequenceName, 2)),
				},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}},
					))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)}}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", "event policies test-sequence are not ready"),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 2 steps with AuthZ enabled, and sequence does have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
					})),
				makeSequenceEventPolicy(sequenceName),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				// Making the event policy for the sequence:
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),

				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Name: resources.SequenceChannelName(sequenceName, 1),
				}, {
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("inmemorychannels"),
					},
					Name: resources.SequenceChannelName(sequenceName, 1),
				},
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceChannelName(sequenceName, 1)),
				},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Object: resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}},
					))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)}}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", "event policies test-sequence are not ready"),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-remove-step with 1 steps with AuthZ enabled, and sequence does have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{})),
				makeSequenceEventPolicy(sequenceName),
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
					}))),
			},
			WantErr: true,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", "Failed to update status for \"test-sequence\": missing field(s): spec.steps"),
			},

			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{})),
			}},
		},
		{
			Name: "sequenceupdate-add-step with 2 steps (3 in total) with AuthZ enabled, and sequence doesn't have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 2),
				resources.NewSubscription(2,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)},
						}))),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)},
						}))),
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequenceupdate-add-step with 2 steps (3 in total) with AuthZ enabled, and sequence does have event policy",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				makeSequenceEventPolicy(sequenceName),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 2),
				resources.NewSubscription(2,
					NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)},
						}))),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 2), 2),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
					},
					Object: resources.NewSubscription(1, NewSequence(sequenceName, testNS,
						WithSequenceChannelTemplateSpec(imc),
						WithSequenceSteps([]v1.SequenceStep{
							{Destination: createDestination(0)},
							{Destination: createDestination(1)},
							{Destination: createDestination(2)},
						}))),
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
						{Destination: createDestination(2)},
					}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s are not ready", sequenceName)),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 2),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequence eventpolicy deleted",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),

				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  v1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceEventPolicyName(sequenceName, "")),
				},
			},

			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled(),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequence with multiple eventpolicies",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				makeSequenceEventPolicy(sequenceName),
				makeSequenceEventPolicy(sequenceName + "-additional-1"),
				makeSequenceEventPolicy(sequenceName + "-additional-2"),
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantCreates: []runtime.Object{
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName+"-additional-1", "")),
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName+"-additional-2", "")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s, %s, %s are not ready", sequenceName, sequenceName+"-additional-1", sequenceName+"-additional-2")),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
					})),
			}},
		},
		{
			Name: "sequence with existing intermediate eventpolicies requiring update and cleanup",
			Key:  pKey,
			Objects: []runtime.Object{
				NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					})),
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				resources.NewSubscription(0, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				resources.NewSubscription(1, NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}))),
				makeSequenceEventPolicy(sequenceName),
				makeSequenceEventPolicy(sequenceName + "-additional"),
				// Existing intermediate policy for input channel (needs update)
				makeInputChannelEventPolicyWithWrongSpec(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
				// Existing intermediate policy for step 0 (correct, no update needed)
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), 0),
				// Existing intermediate policy for step 1 (needs to be deleted)
				makeEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 1), 1),
				// Obsolete policy that should be deleted
				makeObsoleteEventPolicy(sequenceName),
			},
			WantErr: false,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication:       feature.Enabled,
				feature.AuthorizationDefaultMode: feature.AuthorizationAllowSameNamespace,
			}),
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  eventingv1alpha1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Object: makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName, "")),
				},
			},
			WantCreates: []runtime.Object{
				makeInputChannelEventPolicy(sequenceName, resources.SequenceChannelName(sequenceName, 0), resources.SequenceEventPolicyName(sequenceName+"-additional", "")),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  eventingv1alpha1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, resources.SequenceChannelName(sequenceName, 0)),
				},
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  eventingv1alpha1.SchemeGroupVersion.WithResource("eventpolicies"),
					},
					Name: resources.SequenceEventPolicyName(sequenceName, "obsolete"),
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSequence(sequenceName, testNS,
					WithInitSequenceConditions,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{
						{Destination: createDestination(0)},
						{Destination: createDestination(1)},
					}),
					WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithSequenceAddressableNotReady("emptyAddress", "addressable is nil"),
					WithSequenceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithSequenceEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s, %s are not ready", sequenceName, sequenceName+"-additional")),
					WithSequenceChannelStatuses([]v1.SequenceChannelStatus{
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
						{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
								Name:       resources.SequenceChannelName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Channel does not have Ready condition",
							},
						},
					}),
					WithSequenceSubscriptionStatuses([]v1.SequenceSubscriptionStatus{
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
							ReadyCondition: apis.Condition{
								Type:    apis.ConditionReady,
								Status:  corev1.ConditionUnknown,
								Reason:  "NoReady",
								Message: "Subscription does not have Ready condition",
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
			channelableTracker: duck.NewListableTrackerFromTracker(ctx, channelable.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			subscriptionLister: listers.GetSubscriptionLister(),
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			eventPolicyLister:  listers.GetEventPolicyLister(),
		}
		return sequence.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetSequenceLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func makeEventPolicy(sequenceName, channelName string, step int) *eventingv1alpha1.EventPolicy {
	return NewEventPolicy(resources.SequenceEventPolicyName(sequenceName, channelName), testNS,
		WithEventPolicyToRef(channelV1GVK, channelName),
		// from a subscription
		WithEventPolicyFrom(subscriberGVK, resources.SequenceSubscriptionName(sequenceName, step-1), testNS),
		WithEventPolicyOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "flows.knative.dev/v1",
				Kind:       "Sequence",
				Name:       sequenceName,
			},
		}...),
		WithEventPolicyLabels(resources.LabelsForSequenceChannelsEventPolicy(sequenceName)),
	)
}

// Write a function to make the event policy for the sequence
func makeSequenceEventPolicy(sequenceName string) *eventingv1alpha1.EventPolicy {
	return NewEventPolicy(resources.SequenceEventPolicyName(sequenceName, ""), testNS,
		// from a subscription
		WithEventPolicyOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "flows.knative.dev/v1",
				Kind:       "Sequence",
				Name:       sequenceName,
			},
		}...),
	)
}

func makeInputChannelEventPolicy(sequenceName, channelName string, sequenceEventPolicyName string) *eventingv1alpha1.EventPolicy {
	return NewEventPolicy(resources.SequenceEventPolicyName(sequenceName, sequenceEventPolicyName), testNS,
		WithEventPolicyToRef(channelV1GVK, channelName),
		// from a subscription
		WithEventPolicyOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "flows.knative.dev/v1",
				Kind:       "Sequence",
				Name:       sequenceName,
			},
		}...),
		WithEventPolicyLabels(resources.LabelsForSequenceChannelsEventPolicy(sequenceName)),
	)
}

func makeInputChannelEventPolicyWithWrongSpec(sequenceName, channelName, policyName string) *eventingv1alpha1.EventPolicy {
	policy := makeInputChannelEventPolicy(sequenceName, channelName, policyName)
	// Modify the policy to have an incorrect specification
	policy.Spec.From = []eventingv1alpha1.EventPolicySpecFrom{
		{
			Ref: &eventingv1alpha1.EventPolicyFromReference{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
				Name:       "wrong-subscription",
				Namespace:  testNS,
			},
		},
	}

	policy.Spec.To = []eventingv1alpha1.EventPolicySpecTo{
		{
			Ref: &eventingv1alpha1.EventPolicyToReference{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "InMemoryChannel",
				Name:       "wrong-channel",
			},
		},
	}

	return policy
}

func makeObsoleteEventPolicy(sequenceName string) *eventingv1alpha1.EventPolicy {
	return NewEventPolicy(resources.SequenceEventPolicyName(sequenceName, "obsolete"), testNS,
		WithEventPolicyToRef(channelV1GVK, "obsolete-channel"),
		WithEventPolicyOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "flows.knative.dev/v1",
				Kind:       "Sequence",
				Name:       sequenceName,
			},
		}...),
		WithEventPolicyLabels(resources.LabelsForSequenceChannelsEventPolicy(sequenceName)),
	)
}
