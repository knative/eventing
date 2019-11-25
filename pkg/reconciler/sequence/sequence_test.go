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
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	. "knative.dev/eventing/pkg/reconciler/testing"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
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

func createReplyChannel(channelName string) *duckv1beta1.Destination {
	return &duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
			Name:       channelName,
		},
	}
}

func createDeprecatedReplyChannel(channelName string) *duckv1beta1.Destination {
	return &duckv1beta1.Destination{
		DeprecatedAPIVersion: "messaging.knative.dev/v1alpha1",
		DeprecatedKind:       "inmemorychannel",
		DeprecatedName:       channelName,
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

func createDestination(stepNumber int) duckv1beta1.Destination {
	uri := apis.HTTP("example.com")
	uri.Path = fmt.Sprintf("%d", stepNumber)
	return duckv1beta1.Destination{
		URI: uri,
	}
}

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + sequenceName
	imc := &eventingduckv1alpha1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
		},
		Spec: &runtime.RawExtension{Raw: []byte("{}")},
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
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}),
					reconciletesting.WithSequenceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceDeprecatedStatus(),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
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
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceDeprecatedStatus(),
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
					})),
			}},
		}, {
			Name: "singlestepwithdeprecatedreply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createDeprecatedReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createDeprecatedReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}),
					reconciletesting.WithSequenceReply(createDeprecatedReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithSequenceDeprecatedReplyStatus(),
					reconciletesting.WithSequenceDeprecatedStatus(),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
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
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{
						createDestination(0),
						createDestination(1),
						createDestination(2)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(sequenceName, 0),
				createChannel(sequenceName, 1),
				createChannel(sequenceName, 2),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)}))),
				resources.NewSubscription(1, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)}))),
				resources.NewSubscription(2, reconciletesting.NewSequence(sequenceName, testNS, reconciletesting.WithSequenceChannelTemplateSpec(imc), reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{
						createDestination(0),
						createDestination(1),
						createDestination(2),
					}),
					reconciletesting.WithSequenceDeprecatedStatus(),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
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
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{
						createDestination(0),
						createDestination(1),
						createDestination(2)}))},
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
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)}))),
				resources.NewSubscription(1, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)}))),
				resources.NewSubscription(2, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0), createDestination(1), createDestination(2)})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{
						createDestination(0),
						createDestination(1),
						createDestination(2),
					}),
					reconciletesting.WithSequenceDeprecatedStatus(),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 0),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.SequenceSubscriptionName(sequenceName, 1),
								Namespace:  testNS,
							},
						},
						{
							Subscription: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
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
				reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(1)})),
				createChannel(sequenceName, 0),
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(0)}))),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Sequence reconciled"),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{Name: resources.SequenceChannelName(sequenceName, 0)},
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(0, reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(1)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewSequence(sequenceName, testNS,
					reconciletesting.WithInitSequenceConditions,
					reconciletesting.WithSequenceChannelTemplateSpec(imc),
					reconciletesting.WithSequenceSteps([]duckv1beta1.Destination{createDestination(1)}),
					reconciletesting.WithSequenceDeprecatedStatus(),
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
								APIVersion: "messaging.knative.dev/v1alpha1",
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
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			sequenceLister:     listers.GetSequenceLister(),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}, false, logger))
}
