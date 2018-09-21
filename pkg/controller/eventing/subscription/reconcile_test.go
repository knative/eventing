/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package subscription

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	trueVal   = true
	targetURI = "http://target.example.com"
)

const (
	fromChannelName   = "from-channel"
	resultChannelName = "result-channel"
	ChannelKind       = "Channel"
	targetDNS         = "myservice.mynamespace.svc.cluster.local"
	eventType         = "myeventtype"
	subscriptionName  = "test-subscription"
	testNS            = "test-namespace"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

var injectDomainInternalMocks = controllertesting.Mocks{
	MockCreates: []controllertesting.MockCreate{
		func(innerClient client.Client, ctx context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			// If we are creating a Channel, then fill in the status, in particular the DomainInternal as
			// it is used to control whether the Feed is created.
			if channel, ok := obj.(*eventingv1alpha1.Channel); ok {
				err := innerClient.Create(ctx, obj)
				channel.Status.Sinkable = duckv1alpha1.Sinkable{
					DomainInternal: targetDNS,
				}
				return controllertesting.Handled, err
			}
			return controllertesting.Unhandled, nil
		},
	},
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new subscription: adds status, from target resolved",
		InitialState: []runtime.Object{
			getNewFromChannel(),
			getNewResultChannel(),
			getNewSubscription(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, subscriptionName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			//			getActionTargetResolvedFlow(),
			func() *eventingv1alpha1.Channel {
				c := getNewChannel(fromChannelName)
				c.Spec.Channelable = &duckv1alpha1.Channelable{
					Subscribers: []duckv1alpha1.ChannelSubscriberSpec{
						{
							CallableDomain: "dummy",
						},
					},
				}
				return c
			}(),
			getNewSubscription(),
		},
		Mocks: injectDomainInternalMocks,
		Objects: []runtime.Object{&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
				"kind":       "Channel",
				"name":       fromChannelName,
				"status": map[string]interface{}{
					"extra": "fields",
					"fooable": map[string]interface{}{
						"field1": "foo",
						"field2": "bar",
					},
				},
			}}},
	},
	/*
		{
			Name: "new flow: adds status, action target resolved, no flow controller config map, use default 'stub' bus",
			InitialState: []runtime.Object{
				getNewSubscription(),
			},
			ReconcileKey: "test/test-flow",
			WantResult:   reconcile.Result{},
			WantPresent: []runtime.Object{
				//			getActionTargetResolvedFlow(),
				getNewChannel(),
				getNewSubscription(),
				//			getNewFeed(),
			},
			Mocks: injectDomainInternalMocks,
		},
	*/
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()
		r := &reconciler{
			client:        c,
			dynamicClient: dc,
			restConfig:    &rest.Config{},
			recorder:      recorder,
		}
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

/*
func getActionTargetResolvedFlow() *flowsv1alpha1.Flow {
	newFlow := getNewSubscription()
	newFlow.Status = flowsv1alpha1.FlowStatus{
		Conditions: []flowsv1alpha1.FlowCondition{{
			Type:   flowsv1alpha1.FlowConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:    flowsv1alpha1.FlowConditionActionTargetResolved,
			Status:  corev1.ConditionTrue,
			Reason:  "ActionTargetResolved",
			Message: fmt.Sprintf("Resolved to: %q", targetURI),
		}},
	}
	return newFlow
}
*/

func getNewFromChannel() *eventingv1alpha1.Channel {
	return getNewChannel(fromChannelName)
}

func getNewResultChannel() *eventingv1alpha1.Channel {
	return getNewChannel(resultChannelName)
}

func getNewChannel(name string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om("test", name),
		Spec:       eventingv1alpha1.ChannelSpec{},
	}
	channel.ObjectMeta.OwnerReferences = append(channel.ObjectMeta.OwnerReferences, getOwnerReference(false))

	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewSubscription() *eventingv1alpha1.Subscription {
	subscription := &eventingv1alpha1.Subscription{
		TypeMeta:   subscriptionType(),
		ObjectMeta: om(testNS, subscriptionName),
		Spec: eventingv1alpha1.SubscriptionSpec{
			From: corev1.ObjectReference{
				Name:       fromChannelName,
				Kind:       ChannelKind,
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
			//			Channel:    subscriptionName,
			//			Subscriber: targetURI,
		},
	}
	subscription.ObjectMeta.OwnerReferences = append(subscription.ObjectMeta.OwnerReferences, getOwnerReference(false))

	// selflink is not filled in when we create the object, so clear it
	subscription.ObjectMeta.SelfLink = ""
	return subscription
}

/*
func getNewFeed() *feedsv1alpha1.Feed {
	return &feedsv1alpha1.Feed{
		TypeMeta:   feedType(),
		ObjectMeta: feedObjectMeta("test", "test-flow-"),
		Spec: feedsv1alpha1.FeedSpec{
			Action: feedsv1alpha1.FeedAction{
				DNSName: targetDNS,
			},
			Trigger: feedsv1alpha1.EventTrigger{
				EventType:      eventType,
				Resource:       "myresource",
				Service:        "",
				Parameters:     nil,
				ParametersFrom: nil,
			},
		},
	}
}
*/

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func subscriptionType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Subscription",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}
func feedObjectMeta(namespace, generateName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:    namespace,
		GenerateName: generateName,
		OwnerReferences: []metav1.OwnerReference{
			getOwnerReference(true),
		},
	}
}

func getOwnerReference(blockOwnerDeletion bool) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:               "Subscription",
		Name:               subscriptionName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}
