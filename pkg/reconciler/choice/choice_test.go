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

package choice

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
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/choice/resources"
	. "knative.dev/eventing/pkg/reconciler/testing"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
)

const (
	testNS           = "test-namespace"
	choiceName       = "test-choice"
	choiceUID        = "test-choice-uid"
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

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + choiceName
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
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
		}, {
			Name: "singlecase, no filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{{Subscriber: createSubscriber(0)}}),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{{
						FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, with filter",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Filter: createFilter(0), Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Filter: createFilter(0), Subscriber: createSubscriber(0)},
				}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{{Filter: createFilter(0), Subscriber: createSubscriber(0)}}),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{{
						FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, no filter, with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
				}), reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
					}),
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{{
						FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
					}})),
			}},
		}, {
			Name: "singlecase, no filter, with case and global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0), Reply: createCaseReplyChannel(0)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0), Reply: createCaseReplyChannel(0)},
				}), reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0), Reply: createCaseReplyChannel(0)},
					}),
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{{
						FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
						SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
					}})),
			}},
		},
		{
			Name: "two cases, no filters",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				createCaseChannel(choiceName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewFilterSubscription(1, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{
						{
							FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
							FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
							SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						},
						{
							FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 1, corev1.ConditionFalse),
							FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 1, corev1.ConditionFalse),
							SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 1, corev1.ConditionFalse),
						}})),
			}},
		}, {
			Name: "two cases with global reply",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Choice reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(choiceName),
				createCaseChannel(choiceName, 0),
				createCaseChannel(choiceName, 1),
				resources.NewFilterSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(0, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)))),
				resources.NewFilterSubscription(1, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}))),
				resources.NewSubscription(1, reconciletesting.NewChoice(choiceName, testNS, reconciletesting.WithChoiceChannelTemplateSpec(imc), reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
					{Subscriber: createSubscriber(0)},
					{Subscriber: createSubscriber(1)},
				}), reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewChoice(choiceName, testNS,
					reconciletesting.WithInitChoiceConditions,
					reconciletesting.WithChoiceReply(createReplyChannel(replyChannelName)),
					reconciletesting.WithChoiceChannelTemplateSpec(imc),
					reconciletesting.WithChoiceCases([]v1alpha1.ChoiceCase{
						{Subscriber: createSubscriber(0)},
						{Subscriber: createSubscriber(1)},
					}),
					reconciletesting.WithChoiceChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none"),
					reconciletesting.WithChoiceAddressableNotReady("emptyHostname", "hostname is the empty string"),
					reconciletesting.WithChoiceSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none"),
					reconciletesting.WithChoiceIngressChannelStatus(createChoiceChannelStatus(choiceName, corev1.ConditionFalse)),
					reconciletesting.WithChoiceCaseStatuses([]v1alpha1.ChoiceCaseStatus{
						{
							FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
							FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 0, corev1.ConditionFalse),
							SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 0, corev1.ConditionFalse),
						},
						{
							FilterSubscriptionStatus: createChoiceFilterSubscriptionStatus(choiceName, 1, corev1.ConditionFalse),
							FilterChannelStatus:      createChoiceCaseChannelStatus(choiceName, 1, corev1.ConditionFalse),
							SubscriptionStatus:       createChoiceSubscriptionStatus(choiceName, 1, corev1.ConditionFalse),
						}})),
			}},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			choiceLister:       listers.GetChoiceLister(),
			resourceTracker:    fakeResourceTracker{},
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	}, false))
}

func createCaseReplyChannel(caseNumber int) *corev1.ObjectReference {
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

func createChannel(choiceName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ChoiceChannelName(choiceName),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Choice",
						"name":               choiceName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}
}

func createCaseChannel(choiceName string, caseNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.ChoiceCaseChannelName(choiceName, caseNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Choice",
						"name":               choiceName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}
}

func createChoiceCaseChannelStatus(choiceName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ChoiceChannelStatus {
	return v1alpha1.ChoiceChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
			Name:       resources.ChoiceCaseChannelName(choiceName, caseNumber),
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

func createChoiceChannelStatus(choiceName string, status corev1.ConditionStatus) v1alpha1.ChoiceChannelStatus {
	return v1alpha1.ChoiceChannelStatus{
		Channel: corev1.ObjectReference{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
			Name:       resources.ChoiceChannelName(choiceName),
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

func createChoiceFilterSubscriptionStatus(choiceName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ChoiceSubscriptionStatus {
	return v1alpha1.ChoiceSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Subscription",
			Name:       resources.ChoiceFilterSubscriptionName(choiceName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createChoiceSubscriptionStatus(choiceName string, caseNumber int, status corev1.ConditionStatus) v1alpha1.ChoiceSubscriptionStatus {
	return v1alpha1.ChoiceSubscriptionStatus{
		Subscription: corev1.ObjectReference{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Subscription",
			Name:       resources.ChoiceSubscriptionName(choiceName, caseNumber),
			Namespace:  testNS,
		},
	}
}

func createSubscriber(caseNumber int) eventingv1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/%d", caseNumber)
	return eventingv1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}

func createFilter(caseNumber int) *eventingv1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/filter-%d", caseNumber)
	return &eventingv1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}
