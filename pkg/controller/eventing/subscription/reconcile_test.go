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
	"fmt"
	"testing"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
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
	trueVal = true
)

const (
	fromChannelName   = "fromchannel"
	resultChannelName = "resultchannel"
	sourceName        = "source"
	routeName         = "subscriberroute"
	channelKind       = "Channel"
	routeKind         = "Route"
	sourceKind        = "Source"
	targetDNS         = "myfunction.mynamespace.svc.cluster.local"
	sinkableDNS       = "myresultchannel.mynamespace.svc.cluster.local"
	eventType         = "myeventtype"
	subscriptionName  = "testsubscription"
	testNS            = "testnamespace"
	k8sServiceName    = "testk8sservice"
	k8sServiceDNS     = "testk8sservice.testnamespace.svc.cluster.local"
	otherSinkableDNS  = "other-sinkable-channel.mynamespace.svc.cluster.local"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name:    "subscription does not exist",
		WantErr: false,
	}, {
		Name: "subscription but From channel does not exist",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		WantErrMsg: `channels.eventing.knative.dev "fromchannel" not found`,
	}, {
		Name: "subscription, but From is not channelable",
		InitialState: []runtime.Object{
			getNewSourceSubscription(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed. It should actually fail saying that there is no
		// Spec.Subscribers field.
		WantErrMsg: "invalid JSON document",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       sourceKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sourceName,
					},
					"spec": map[string]interface{}{},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "Valid channel, subscriber does not exist",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		WantErrMsg: `routes.serving.knative.dev "subscriberroute" not found`,
		WantPresent: []runtime.Object{
			getNewSubscriptionWithUnknownConditions(),
		},
		Scheme: scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
		},
	}, {
		Name: "Valid channel, subscriber is not targetable",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		WantPresent: []runtime.Object{
			getNewSubscriptionWithUnknownConditions(),
		},
		WantErrMsg: "status does not contain targetable",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"someotherstuff": targetDNS,
					},
				},
			},
		},
	}, {
		Name: "Valid channel and subscriber, result does not exist",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		WantPresent: []runtime.Object{
			getNewSubscriptionWithUnknownConditionsAndPhysicalSubscriber(),
		},
		WantErrMsg: `channels.eventing.knative.dev "resultchannel" not found`,
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "valid channel, subscriber, result is not sinkable",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		WantErrMsg: "status does not contain sinkable",
		WantPresent: []runtime.Object{
			// TODO: Again this works on gke cluster, but I need to set
			// something else up here. later...
			// getNewSubscriptionWithReferencesResolvedStatus(),
			getNewSubscriptionWithUnknownConditionsAndPhysicalSubscriber(),
		},
		Scheme: scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
		},
	}, {
		Name: "new subscription: adds status, all targets resolved, subscribers modified",
		InitialState: []runtime.Object{
			getNewSubscription(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberReply(),
		},
		WantErrMsg: "invalid JSON document",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "new subscription: adds status, all targets resolved, subscribers modified -- empty but non-nil reply",
		InitialState: []runtime.Object{
			getNewSubscriptionWithEmptyNonNilReply(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberAndNoReply(),
		},
		WantErrMsg: "invalid JSON document",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "new subscription: adds status, all targets resolved, subscribers modified -- empty but non-nil subscriber",
		InitialState: []runtime.Object{
			getNewSubscriptionWithEmptyNonNilSubscriber(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewSubscriptionWithReferencesResolvedAndPhysicalReplyAndNoSubscriber(),
		},
		WantErrMsg: "invalid JSON document",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "new subscription to K8s Service: adds status, all targets resolved, subscribers modified",
		InitialState: []runtime.Object{
			getNewSubscriptionToK8sService(),
			getK8sService(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewSubscriptionToK8sServiceWithReferencesResolvedAndPhysicalFromSubscriberReply(),
		},
		WantErrMsg: "invalid JSON document",
		Scheme:     scheme.Scheme,
		Objects: []runtime.Object{
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using K8s Service)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      k8sServiceName,
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	}, {
		Name: "new subscription with from channel: adds status, all targets resolved, subscribers modified",
		InitialState: []runtime.Object{
			getNewSubscriptionWithFromChannel(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantErrMsg: "invalid JSON document",
		WantPresent: []runtime.Object{
			getNewSubscriptionWithSourceWithReferencesResolvedAndPhysicalFromSubscriberReply(),
		},
		Scheme: scheme.Scheme,
		Objects: []runtime.Object{
			// Source with a reference to the From Channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       sourceKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sourceName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	},
	{
		Name: "sync multiple Subscriptions to one channel",
		InitialState: []runtime.Object{
			// The first two Subscriptions both have the same physical From, so we should see that
			// Channel updated with both Subscriptions.
			getNewSubscriptionWithFromChannel(),
			rename(getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberReply()),
			// This subscription has a different physical From, so we should not see it in the same
			// Channel as the first two.
			getSubscriptionWithDifferentChannel(),
		},
		// TODO: JSON patch is not working on the fake, see
		// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
		// failure for now, until upstream is fixed.
		WantResult: reconcile.Result{},
		WantErrMsg: "invalid JSON document",
		WantPresent: []runtime.Object{
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. The entire test is really to
			// verify the following, but can't be done because the call to Patch fails (it assumes
			// a Strategic Merge Patch, whereas we are doing a JSON Patch). so for now, comment it
			// out.
			//getChannelWithMultipleSubscriptions(),
			getNewSubscriptionWithSourceWithReferencesResolvedAndPhysicalFromSubscriberReply(),
			// Unaltered because this Subscription was not reconciled.
			rename(getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberReply()),
			getSubscriptionWithDifferentChannel(),
		},
		Scheme: scheme.Scheme,
		Objects: []runtime.Object{
			// Source with a reference to the From Channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       sourceKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sourceName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Source channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      fromChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
				},
			},
			// Subscriber (using knative route)
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "serving.knative.dev/v1alpha1",
					"kind":       routeKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      routeName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": targetDNS,
						},
					},
				},
			},
			// Reply channel
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": eventingv1alpha1.SchemeGroupVersion.String(),
					"kind":       channelKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      resultChannelName,
					},
					"spec": map[string]interface{}{
						"channelable": map[string]interface{}{},
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
	},
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
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, subscriptionName)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getNewFromChannel() *eventingv1alpha1.Channel {
	return getNewChannel(fromChannelName)
}

func getNewReplyChannel() *eventingv1alpha1.Channel {
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

// rename renames the subscription. It is intended to be used in tests that create multiple
// Subscriptions, so that there are no naming conflicts.
func rename(sub *eventingv1alpha1.Subscription) *eventingv1alpha1.Subscription {
	sub.Name = "renamed"
	sub.UID = "renamed-UID"
	sub.Status.PhysicalSubscription.SubscriberURI = ""
	sub.Status.PhysicalSubscription.ReplyURI = otherSinkableDNS
	return sub
}

func getNewSubscription() *eventingv1alpha1.Subscription {
	subscription := &eventingv1alpha1.Subscription{
		TypeMeta:   subscriptionType(),
		ObjectMeta: om(testNS, subscriptionName),
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				Name:       fromChannelName,
				Kind:       channelKind,
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
			Subscriber: &eventingv1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					Name:       routeName,
					Kind:       routeKind,
					APIVersion: "serving.knative.dev/v1alpha1",
				},
			},
			Reply: &eventingv1alpha1.ReplyStrategy{
				Channel: &corev1.ObjectReference{
					Name:       resultChannelName,
					Kind:       channelKind,
					APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				},
			},
		},
	}
	subscription.ObjectMeta.OwnerReferences = append(subscription.ObjectMeta.OwnerReferences, getOwnerReference(false))

	// selflink is not filled in when we create the object, so clear it
	subscription.ObjectMeta.SelfLink = ""
	return subscription
}

func getNewSubscriptionWithEmptyNonNilReply() *eventingv1alpha1.Subscription {
	sub := getNewSubscription()
	sub.Spec.Reply = &eventingv1alpha1.ReplyStrategy{}
	return sub
}

func getNewSubscriptionWithEmptyNonNilSubscriber() *eventingv1alpha1.Subscription {
	sub := getNewSubscription()
	sub.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{}
	return sub
}

func getNewSourceSubscription() *eventingv1alpha1.Subscription {
	sub := getNewSubscription()
	sub.Spec.Channel = corev1.ObjectReference{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       sourceKind,
		Name:       sourceName,
	}
	return sub
}

func getNewSubscriptionToK8sService() *eventingv1alpha1.Subscription {
	sub := getNewSubscription()
	sub.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
		Ref: &corev1.ObjectReference{
			Name:       k8sServiceName,
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
	return sub
}

func getNewSubscriptionWithFromChannel() *eventingv1alpha1.Subscription {
	subscription := &eventingv1alpha1.Subscription{
		TypeMeta:   subscriptionType(),
		ObjectMeta: om(testNS, subscriptionName),
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				Name:       fromChannelName,
				Kind:       channelKind,
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
			Subscriber: &eventingv1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					Name:       routeName,
					Kind:       routeKind,
					APIVersion: "serving.knative.dev/v1alpha1",
				},
			},
			Reply: &eventingv1alpha1.ReplyStrategy{
				Channel: &corev1.ObjectReference{
					Name:       resultChannelName,
					Kind:       channelKind,
					APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				},
			},
		},
	}
	subscription.ObjectMeta.OwnerReferences = append(subscription.ObjectMeta.OwnerReferences, getOwnerReference(false))

	// selflink is not filled in when we create the object, so clear it
	subscription.ObjectMeta.SelfLink = ""
	return subscription
}

func getNewSubscriptionWithUnknownConditions() *eventingv1alpha1.Subscription {
	s := getNewSubscription()
	s.Status.InitializeConditions()
	return s
}
func getNewSubscriptionWithUnknownConditionsAndPhysicalSubscriber() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithUnknownConditions()
	s.Status.PhysicalSubscription.SubscriberURI = domainToURL(targetDNS)
	return s
}

func getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberAndNoReply() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithEmptyNonNilReply()
	s.Status.InitializeConditions()
	s.Status.MarkReferencesResolved()
	s.Status.PhysicalSubscription.SubscriberURI = domainToURL(targetDNS)
	return s
}

func getNewSubscriptionWithReferencesResolvedAndPhysicalReplyAndNoSubscriber() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithEmptyNonNilSubscriber()
	s.Status.InitializeConditions()
	s.Status.MarkReferencesResolved()
	s.Status.PhysicalSubscription.ReplyURI = domainToURL(sinkableDNS)
	return s
}

func getNewSubscriptionWithReferencesResolvedAndPhysicalSubscriberReply() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithUnknownConditions()
	s.Status.MarkReferencesResolved()
	s.Status.PhysicalSubscription.SubscriberURI = domainToURL(targetDNS)
	s.Status.PhysicalSubscription.ReplyURI = domainToURL(sinkableDNS)
	return s
}

func getNewSubscriptionToK8sServiceWithReferencesResolvedAndPhysicalFromSubscriberReply() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionToK8sService()
	s.Status.InitializeConditions()
	s.Status.MarkReferencesResolved()
	s.Status.PhysicalSubscription = eventingv1alpha1.SubscriptionStatusPhysicalSubscription{
		SubscriberURI: domainToURL(k8sServiceDNS),
		ReplyURI:      domainToURL(sinkableDNS),
	}
	return s
}

func getNewSubscriptionWithSourceWithReferencesResolvedAndPhysicalFromSubscriberReply() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithFromChannel()
	s.Status.InitializeConditions()
	s.Status.MarkReferencesResolved()
	s.Status.PhysicalSubscription = eventingv1alpha1.SubscriptionStatusPhysicalSubscription{
		SubscriberURI: domainToURL(targetDNS),
		ReplyURI:      domainToURL(sinkableDNS),
	}
	return s
}

func getNewSubscriptionWithReferencesResolvedStatus() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithUnknownConditions()
	s.Status.MarkReferencesResolved()
	return s
}

func getSubscriptionWithDifferentChannel() *eventingv1alpha1.Subscription {
	s := getNewSubscriptionWithSourceWithReferencesResolvedAndPhysicalFromSubscriberReply()
	s.Name = "different-channel"
	s.UID = "different-channel-UID"
	s.Status.PhysicalSubscription.SubscriberURI = "some-other-domain"
	return s
}

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

func getK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      k8sServiceName,
		},
	}
}

func getChannelWithMultipleSubscriptions() *eventingv1alpha1.Channel {
	return &eventingv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       channelKind,
		},
		ObjectMeta: om(testNS, fromChannelName),
		Spec: eventingv1alpha1.ChannelSpec{
			Channelable: &eventingduck.Channelable{
				Subscribers: []eventingduck.ChannelSubscriberSpec{
					{
						SubscriberURI: targetDNS,
						ReplyURI:      sinkableDNS,
					},
					{
						ReplyURI: otherSinkableDNS,
					},
				},
			},
		},
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
