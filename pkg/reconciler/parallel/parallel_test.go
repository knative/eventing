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

	"knative.dev/eventing/pkg/client/injection/reconciler/flows/v1alpha1/parallel"

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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
	. "knative.dev/eventing/pkg/reconciler/testing"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
)

const (
	testNS             = "test-namespace"
	parallelName       = "test-parallel"
	parallelUID        = "test-parallel-uid"
	replyChannelName   = "reply-channel"
	parallelGeneration = 79
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestAllBranches(t *testing.T) {
	pKey := testNS + "/" + parallelName
	imc := &eventingduck.ChannelTemplateSpec{
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
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelDeleted)},
			WantErr: false,
		}, {
			Name: "single branch, no filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelGeneration(parallelGeneration),
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelGeneration(parallelGeneration),
					reconciletesting.WithFlowsParallelStatusObservedGeneration(parallelGeneration),
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{{Subscriber: createSubscriber(0)}}),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch, with filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Filter: createFilter(0), Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{{Filter: createFilter(0), Subscriber: createSubscriber(0)}}),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch, no filter, with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}), reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}),
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "single branch with reply, no filter, with case and global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
				}), reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}),
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "two branches, no filters",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewFilterSubscription(1, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{
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
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
				resources.NewFilterSubscription(1, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{
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
				reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(1)},
					})),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					})))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ParallelReconciled", `Parallel reconciled: "test-namespace/test-parallel"`),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{Name: resources.ParallelBranchChannelName(parallelName, 0)},
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewFlowsParallel(parallelName, testNS, reconciletesting.WithFlowsParallelChannelTemplateSpec(imc), reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(1)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewFlowsParallel(parallelName, testNS,
					reconciletesting.WithInitFlowsParallelConditions,
					reconciletesting.WithFlowsParallelChannelTemplateSpec(imc),
					reconciletesting.WithFlowsParallelBranches([]v1alpha1.ParallelBranch{{Subscriber: createSubscriber(1)}}),
					reconciletesting.WithFlowsParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelAddressableNotReady("emptyAddress", "addressable is nil"),
					reconciletesting.WithFlowsParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithFlowsParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithFlowsParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
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
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			parallelLister:     listers.GetFlowsParallelLister(),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			subscriptionLister: listers.GetSubscriptionLister(),
		}
		return parallel.NewReconciler(ctx, r.Logger, r.EventingClientSet, listers.GetFlowsParallelLister(), r.Recorder, r)
	}, false, logger))
}

func createBranchReplyChannel(caseNumber int) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
			Name:       fmt.Sprintf("%s-case-%d", replyChannelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createReplyChannel(channelName string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
			Name:       channelName,
			Namespace:  testNS,
		},
	}
}

func createChannel(parallelName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ParallelChannelName(parallelName),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1alpha1",
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
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ParallelBranchChannelName(parallelName, caseNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "flows.knative.dev/v1alpha1",
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

func createParallelBranchChannelStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ParallelChannelStatus {
	return v1alpha1.ParallelChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
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

func createParallelChannelStatus(parallelName string, status corev1.ConditionStatus) v1alpha1.ParallelChannelStatus {
	return v1alpha1.ParallelChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
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

func createParallelFilterSubscriptionStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ParallelSubscriptionStatus {
	return v1alpha1.ParallelSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "Subscription",
			Name:       resources.ParallelFilterSubscriptionName(parallelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createParallelSubscriptionStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ParallelSubscriptionStatus {
	return v1alpha1.ParallelSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
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
