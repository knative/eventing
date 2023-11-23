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
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
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

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

const (
	testNS             = "test-namespace"
	sequenceName       = "test-sequence"
	replyChannelName   = "reply-channel"
	sequenceGeneration = 7
)

var (
	subscriberGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Subscriber",
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
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
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
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
				}),
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
				}),
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
				}),
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
			},
			Name: resources.SequenceChannelName(sequenceName, 0),
		}},
		WantCreates: []runtime.Object{
			resources.NewSubscription(0, NewSequence(sequenceName, testNS,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(1)}}))),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(1)}}),
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
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
				}),
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
				}),
				WithSequenceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
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
		Name: "OIDC: creates OIDC service account",
		Key:  pKey,
		Ctx: feature.ToContext(context.Background(), feature.Flags{
			feature.OIDCAuthentication: feature.Enabled,
		}),
		Objects: []runtime.Object{
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})))},
		WantErr: false,
		WantCreates: []runtime.Object{
			makeSequenceOIDCServiceAccount(),
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
				WithSequenceOIDCIdentityCreatedSucceeded(),
				WithSequenceOIDCServiceAccountName(makeSequenceOIDCServiceAccount().Name)),
		}},
	}, {
		Name: "OIDC: Sequence not ready on invalid OIDC service account",
		Key:  pKey,
		Ctx: feature.ToContext(context.Background(), feature.Flags{
			feature.OIDCAuthentication: feature.Enabled,
		}),
		Objects: []runtime.Object{
			makeSequenceOIDCServiceAccountWithoutOwnerRef(),
			NewSequence(sequenceName, testNS,
				WithInitSequenceConditions,
				WithSequenceChannelTemplateSpec(imc),
				WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})),
			createChannel(sequenceName, 0),
			resources.NewSubscription(0,
				NewSequence(sequenceName, testNS,
					WithSequenceChannelTemplateSpec(imc),
					WithSequenceSteps([]v1.SequenceStep{{Destination: createDestination(0)}})))},
		WantErr:     true,
		WantCreates: []runtime.Object{},
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
				WithSequenceOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", fmt.Sprintf("service account %s not owned by Sequence %s", makeSequenceOIDCServiceAccountWithoutOwnerRef().Name, sequenceName)),
				WithSequenceOIDCServiceAccountName(makeSequenceOIDCServiceAccountWithoutOwnerRef().Name)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("service account %s not owned by Sequence %s", makeSequenceOIDCServiceAccountWithoutOwnerRef().Name, sequenceName)),
		},
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		r := &Reconciler{
			sequenceLister:       listers.GetSequenceLister(),
			channelableTracker:   duck.NewListableTrackerFromTracker(ctx, channelable.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			subscriptionLister:   listers.GetSubscriptionLister(),
			eventingClientSet:    fakeeventingclient.Get(ctx),
			dynamicClientSet:     fakedynamicclient.Get(ctx),
			kubeclient:           fakekubeclient.Get(ctx),
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		return sequence.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetSequenceLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func makeSequenceOIDCServiceAccount() *corev1.ServiceAccount {
	return auth.GetOIDCServiceAccountForResource(v1.SchemeGroupVersion.WithKind("Sequence"), metav1.ObjectMeta{
		Name:      sequenceName,
		Namespace: testNS,
	})
}

func makeSequenceOIDCServiceAccountWithoutOwnerRef() *corev1.ServiceAccount {
	sa := auth.GetOIDCServiceAccountForResource(v1.SchemeGroupVersion.WithKind("Sequence"), metav1.ObjectMeta{
		Name:      sequenceName,
		Namespace: testNS,
	})
	sa.OwnerReferences = nil

	return sa
}
