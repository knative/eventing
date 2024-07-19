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

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

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
		Kind:    "Subscriber",
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

	table := TableTest{{
		Name: "threestep with AuthZ enabled",
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
		WithEventPolicyFromSub(resources.SequenceSubscriptionName(sequenceName, step-1)), // Previous step's subscription
		WithEventPolicyOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "flows.knative.dev/v1",
				Kind:       "Sequence",
				Name:       sequenceName,
			},

			// FIXME: The testing and expected test result have difference in the OwnerReferences field.
		}...),
		WithEventPolicyLabels(resources.LabelsForSequenceChannelsEventPolicy(sequenceName)),
	)
}
