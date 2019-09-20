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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
	. "knative.dev/eventing/pkg/reconciler/testing"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
)

const (
	testNS           = "test-namespace"
	parallelName     = "test-parallel"
	parallelUID      = "test-parallel-uid"
	replyChannelName = "reply-channel"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

type fakeAddressableInformer struct{}

func (*fakeAddressableInformer) NewTracker(callback func(string), lease time.Duration) duck.ResourceTracker {
	return fakeResourceTracker{}
}

type fakeResourceTracker struct{}

func (fakeResourceTracker) TrackInNamespace(metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error { return nil }
}

func (fakeResourceTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	return nil
}

func (fakeResourceTracker) OnChanged(obj interface{}) {
}

func TestAllBranches(t *testing.T) {
	pKey := testNS + "/" + parallelName
	imc := &eventingduck.ChannelTemplateSpec{
		metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
		},
		&runtime.RawExtension{Raw: []byte("{}")},
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
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
		}, {
			Name: "singlecase, no filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{{Subscriber: createSubscriber(0)}}),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, with filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Filter: createFilter(0), Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{{Filter: createFilter(0), Subscriber: createSubscriber(0)}}),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, no filter, with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}), reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
					}),
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, no filter, with case and global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
				}), reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0), Reply: createBranchReplyChannel(0)},
					}),
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{{
						FilterSubscriptionStatus: createParallelFilterSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createParallelBranchChannelStatus(parallelName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createParallelSubscriptionStatus(parallelName, 0, corev1.ConditionFalse),
					}})),
			}},
		},
		{
			Name: "two cases, no filters",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewFilterSubscription(1, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{
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
			Name: "two cases with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Parallel reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(parallelName),
				createBranchChannel(parallelName, 0),
				createBranchChannel(parallelName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)))),
				resources.NewFilterSubscription(1, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewParallel(parallelName, testNS, reconciletesting.WithParallelChannelTemplateSpec(imc), reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewParallel(parallelName, testNS,
					reconciletesting.WithInitParallelConditions,
					reconciletesting.WithParallelReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithParallelChannelTemplateSpec(imc),
					reconciletesting.WithParallelBranches([]v1alpha1.ParallelBranch{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithParallelChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithParallelAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithParallelSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithParallelIngressChannelStatus(createParallelChannelStatus(parallelName, corev1.ConditionFalse)),
					reconciletesting.WithParallelBranchStatuses([]v1alpha1.ParallelBranchStatus{
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
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			parallelLister:     listers.GetParallelLister(),
			resourceTracker:    fakeResourceTracker{},
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}, false, logger))
}

func createBranchReplyChannel(caseNumber int) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "inmemorychannel",
		Name:       fmt.Sprintf("%s-case-%d", replyChannelName, caseNumber),
	}
}

func createReplyChannel(channelName string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "inmemorychannel",
		Name:       channelName,
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
						"apiVersion":         "messaging.knative.dev/v1alpha1",
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
						"apiVersion":         "messaging.knative.dev/v1alpha1",
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
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Subscription",
			Name:       resources.ParallelFilterSubscriptionName(parallelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createParallelSubscriptionStatus(parallelName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ParallelSubscriptionStatus {
	return v1alpha1.ParallelSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Subscription",
			Name:       resources.ParallelSubscriptionName(parallelName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createSubscriber(caseNumber int) v1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/%d", caseNumber)
	return v1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}

func createFilter(caseNumber int) *v1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/filter-%d", caseNumber)
	return &v1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}
