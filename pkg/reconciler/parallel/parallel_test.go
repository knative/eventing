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

package parallel

import (
	"context"
	"fmt"
	"testing"

	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/parallel"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	v1 "knative.dev/eventing/pkg/apis/flows/v1"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
)

const (
	testNS             = "test-namespace"
	parallelName       = "test-parallel"
	replyChannelName   = "reply-channel"
	parallelGeneration = 79
)

var (
	subscriberGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Subscriber",
	}
)

func TestAllBranches(t *testing.T) {
	pKey := testNS + "/" + parallelName
	imc := &messagingv1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
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
			//				NewTrigger(triggerName, testNS),
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
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelDeleted)},
			WantErr: false,
		}, {
			Name: "single branch, no filter",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelGeneration(parallelGeneration),
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelGeneration(parallelGeneration),
					WithFlowsParallelStatusObservedGeneration(parallelGeneration),
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{{Subscriber: createSubscriber(0)}}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch, with filter",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Filter: createFilter(0), Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{{Filter: createFilter(0), Subscriber: createSubscriber(0)}}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch, with filter, with delivery",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Filter: createFilter(0), Subscriber: createSubscriber(0), Delivery: createDelivery(subscriberGVK, "dlc", testNS)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0), Delivery: createDelivery(subscriberGVK, "dlc", testNS)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{{Filter: createFilter(0), Subscriber: createSubscriber(0), Delivery: createDelivery(subscriberGVK, "dlc", testNS)}}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch, no filter, with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}), WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}),
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch with reply, no filter, with case and global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
				}), WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}),
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "two branches, no filters",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewFilterSubscription(1, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{
						{
							FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
							FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
							SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						},
						{
							FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 1, corev1.ConditionFalse),
							FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 1, corev1.ConditionFalse),
							SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 1, corev1.ConditionFalse),
						}})),
			}},
		}, {
			Name: "two branches with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
				resources.NewFilterSubscription(1, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{
						{
							FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
							FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
							SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						},
						{
							FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 1, corev1.ConditionFalse),
							FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 1, corev1.ConditionFalse),
							SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 1, corev1.ConditionFalse),
						}})),
			}},
		},
		{
			Name: "single branch, no filter, update subscription",
			Key:  pKey,
			Objects: []runtime.Object{
				NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(1)},
					})),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					})))},
			WantErr: false,
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  v1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: resources.ParallelBranchChannelName(parallelName, 0),
			}},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, NewFlowsParallel(parallelName, testNS, WithFlowsParallelChannelTemplateSpec(imc), WithFlowsParallelBranches([]v1.ParallelBranch{
					{Subscriber: createSubscriber(1)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewFlowsParallel(parallelName, testNS,
					WithInitFlowsParallelConditions,
					WithFlowsParallelChannelTemplateSpec(imc),
					WithFlowsParallelBranches([]v1.ParallelBranch{{Subscriber: createSubscriber(1)}}),
					WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					WithFlowsParallelBranchStatuses([]v1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		r := &Reconciler{
			parallelLister:     listers.GetParallelLister(),
			channelableTracker: duck.NewListableTrackerFromTracker(ctx, channelable.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			subscriptionLister: listers.GetSubscriptionLister(),
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
		}
		return parallel.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetParallelLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func createBranchReplyChannel(caseNumber int) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
			Name:       fmt.Sprintf("%s-case-%d", replyChannelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

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

func createChannel(parallelName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ParallelChannelName(parallelName),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Parallel",
						"name":               parallelName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}
}

func createBranchChannel(parallelName string, caseNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ParallelBranchChannelName(parallelName, caseNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Parallel",
						"name":               parallelName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}
}

func createParallelBranchChannelStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1.ParallelChannelStatus {
	return v1.ParallelChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
			Name:       resources.ParallelBranchChannelName(parallelName, caseNumber),
			Namespace:  testNS,
		},
		ReadyCondition: apis.Condition{
			Type:    apis.ConditionReady,
			Status:  status,
			Reason:  "NotAddressable",
			Message: "Channel is not addressable",
		},
	}
}

func createParallelChannelStatus(parallelName string, status corev1.ConditionStatus) v1.ParallelChannelStatus {
	return v1.ParallelChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
			Name:       resources.ParallelChannelName(parallelName),
			Namespace:  testNS,
		},
		ReadyCondition: apis.Condition{
			Type:    apis.ConditionReady,
			Status:  status,
			Reason:  "NotAddressable",
			Message: "Channel is not addressable",
		},
	}
}

func createParallelFilterSubscriptionStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1.ParallelSubscriptionStatus {
	return v1.ParallelSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Subscription",
			Name:       resources.ParallelFilterSubscriptionName(parallelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createParallelSubscriptionStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1.ParallelSubscriptionStatus {
	return v1.ParallelSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Subscription",
			Name:       resources.ParallelSubscriptionName(parallelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createSubscriber(caseNumber int) duckv1.Destination {
	uri := apis.HTTP(fmt.Sprintf("example.com/%d", caseNumber))
	return duckv1.Destination{
		URI: uri,
	}
}

func createFilter(caseNumber int) *duckv1.Destination {
	uri := apis.HTTP(fmt.Sprintf("example.com/filter-%d", caseNumber))
	return &duckv1.Destination{
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
