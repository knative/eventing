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

package flow

import (
	"context"
	"fmt"
	"testing"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsv1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/system"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	trueVal   = true
	falseVal  = false
	targetURI = "http://target.example.com"
)

const (
	targetDNS = "myservice.mynamespace.svc.cluster.local"
	eventType = "myeventtype"
	flowName  = "test-flow"
)

func init() {
	// Add types to scheme
	feedsv1alpha1.AddToScheme(scheme.Scheme)
	flowsv1alpha1.AddToScheme(scheme.Scheme)
	channelsv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new flow: adds status, action target resolved",
		InitialState: []runtime.Object{
			getNewFlow(),
			getFlowControllerConfigMap(),
		},
		ReconcileKey: "test/test-flow",
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getActionTargetResolvedFlow(),
			func() *channelsv1alpha1.Channel {
				c := getNewChannel()
				c.Spec.ClusterBus = "special-bus"
				return c
			}(),
			getNewSubscription(),
			getNewFeed(),
		},
	},
	{
		Name: "new flow: adds status, action target resolved, no flow controller config map, use default 'stub' bus",
		InitialState: []runtime.Object{
			getNewFlow(),
		},
		ReconcileKey: "test/test-flow",
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getActionTargetResolvedFlow(),
			getNewChannel(),
			getNewSubscription(),
			getNewFeed(),
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		r := &reconciler{
			client:   &wrappedClient{client: tc.GetClient()},
			recorder: recorder,
		}
		t.Run(tc.Name, tc.Runner(t, r, r.client))
	}
}

func getActionTargetResolvedFlow() *flowsv1alpha1.Flow {
	newFlow := getNewFlow()
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

func getNewFlow() *flowsv1alpha1.Flow {
	return &flowsv1alpha1.Flow{
		TypeMeta:   flowType(),
		ObjectMeta: om("test", flowName),
		Spec: flowsv1alpha1.FlowSpec{
			Action: flowsv1alpha1.FlowAction{
				TargetURI: &targetURI,
			},
			Trigger: flowsv1alpha1.EventTrigger{
				EventType:      eventType,
				Resource:       "myresource",
				Service:        "",
				Parameters:     nil,
				ParametersFrom: nil,
			},
		},
	}
}

func getNewChannel() *channelsv1alpha1.Channel {
	channel := &channelsv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om("test", flowName),
		Spec: channelsv1alpha1.ChannelSpec{
			ClusterBus: "stub",
		},
	}
	channel.ObjectMeta.OwnerReferences = append(channel.ObjectMeta.OwnerReferences, getOwnerReference())

	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewSubscription() *channelsv1alpha1.Subscription {
	subscription := &channelsv1alpha1.Subscription{
		TypeMeta:   subscriptionType(),
		ObjectMeta: om("test", flowName),
		Spec: channelsv1alpha1.SubscriptionSpec{
			Channel:    flowName,
			Subscriber: targetURI,
		},
	}
	subscription.ObjectMeta.OwnerReferences = append(subscription.ObjectMeta.OwnerReferences, getOwnerReference())

	// selflink is not filled in when we create the object, so clear it
	subscription.ObjectMeta.SelfLink = ""
	return subscription
}

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

func getFlowControllerConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: om(system.Namespace, controllerConfigMapName),
		Data: map[string]string{
			defaultClusterBusConfigMapKey: "special-bus",
		},
	}
}

func flowType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: flowsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Flow",
	}
}

func feedType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: feedsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Feed",
	}
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: channelsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func subscriptionType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: channelsv1alpha1.SchemeGroupVersion.String(),
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
			getOwnerReference(),
		},
	}
}

func getOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flowsv1alpha1.SchemeGroupVersion.String(),
		Kind:               "Flow",
		Name:               flowName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &falseVal,
	}
}

// wrappedClient is a wrapper around a FakeClient. It fills in created Channel's
// status.domainInternal field, whose value is used to determine code flow in reconcile.go.
// TODO: Replace this with Reactors, once the FakeClient uses Reactors.
type wrappedClient struct {
	client client.Client
}

// Assert that wrappedClient can act as a client.Client.
var _ client.Client = &wrappedClient{}

func (w *wrappedClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return w.client.Get(ctx, key, obj)
}

func (w *wrappedClient) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	return w.client.List(ctx, opts, list)
}

func (w *wrappedClient) Create(ctx context.Context, obj runtime.Object) error {
	err := w.client.Create(ctx, obj)
	if err != nil {
		return err
	}

	// If we are creating a Channel, then fill in the status, in particular the DomainInternal as
	// it used to control whether the Feed is created.
	if channel, ok := obj.(*channelsv1alpha1.Channel); ok {
		channel.Status.DomainInternal = targetDNS
	}
	return nil
}

func (w *wrappedClient) Delete(ctx context.Context, obj runtime.Object) error {
	return w.client.Delete(ctx, obj)
}

func (w *wrappedClient) Update(ctx context.Context, obj runtime.Object) error {
	return w.client.Update(ctx, obj)
}
