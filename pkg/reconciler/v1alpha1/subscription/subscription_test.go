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

	"go.uber.org/zap"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/eventing/pkg/utils/resolve"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	trueVal = true

	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		subscriptionReconciled:         {Reason: subscriptionReconciled, Type: corev1.EventTypeNormal},
		subscriptionUpdateStatusFailed: {Reason: subscriptionUpdateStatusFailed, Type: corev1.EventTypeWarning},
		physicalChannelSyncFailed:      {Reason: physicalChannelSyncFailed, Type: corev1.EventTypeWarning},
		channelReferenceFetchFailed:    {Reason: channelReferenceFetchFailed, Type: corev1.EventTypeWarning},
		subscriberResolveFailed:        {Reason: subscriberResolveFailed, Type: corev1.EventTypeWarning},
		resultResolveFailed:            {Reason: resultResolveFailed, Type: corev1.EventTypeWarning},
	}
)

const (
	fromChannelName   = "fromchannel"
	resultChannelName = "resultchannel"
	sourceName        = "source"
	routeName         = "subscriberroute"
	channelKind       = "Channel"
	routeKind         = "Route"
	sourceKind        = "Source"
	subscriptionKind  = "Subscription"
	eventType         = "myeventtype"
	subscriptionName  = "testsubscription"
	testNS            = "testnamespace"
	k8sServiceName    = "testk8sservice"
)

var (
	targetDNS           = "myfunction.mynamespace.svc." + utils.GetClusterDomainName()
	sinkableDNS         = "myresultchannel.mynamespace.svc." + utils.GetClusterDomainName()
	k8sServiceDNS       = "testk8sservice.testnamespace.svc." + utils.GetClusterDomainName()
	otherAddressableDNS = "other-sinkable-channel.mynamespace.svc." + utils.GetClusterDomainName()
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:    "subscription does not exist",
			WantErr: false,
		}, {
			Name: "subscription but From channel does not exist",
			InitialState: []runtime.Object{
				Subscription(),
			},
			WantErrMsg: `channels.eventing.knative.dev "fromchannel" not found`,
			WantEvent: []corev1.Event{
				events[channelReferenceFetchFailed],
			},
		}, {
			Name: "subscription, but From is not subscribable",
			InitialState: []runtime.Object{
				Subscription().FromSource(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed. It should actually fail saying that there is no
			// Spec.Subscribers field.
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "Valid channel, subscriber does not exist",
			InitialState: []runtime.Object{
				Subscription(),
			},
			WantErrMsg: `routes.serving.knative.dev "subscriberroute" not found`,
			WantPresent: []runtime.Object{
				Subscription().UnknownConditions(),
			},
			WantEvent: []corev1.Event{
				events[subscriberResolveFailed],
			},
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
							"subscribable": map[string]interface{}{},
						},
					},
				},
			},
		}, {
			Name: "Valid channel, subscriber is not callable",
			InitialState: []runtime.Object{
				Subscription(),
			},
			WantPresent: []runtime.Object{
				Subscription().UnknownConditions(),
			},
			WantErrMsg: "status does not contain address",
			WantEvent: []corev1.Event{
				events[subscriberResolveFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
				Subscription(),
			},
			WantPresent: []runtime.Object{
				Subscription().UnknownConditions().PhysicalSubscriber(targetDNS),
			},
			WantErrMsg: `channels.eventing.knative.dev "resultchannel" not found`,
			WantEvent: []corev1.Event{
				events[resultResolveFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "valid channel, subscriber, result is not addressable",
			InitialState: []runtime.Object{
				Subscription(),
			},
			WantErrMsg: "status does not contain address",
			WantPresent: []runtime.Object{
				// TODO: Again this works on gke cluster, but I need to set
				// something else up here. later...
				// Subscription().ReferencesResolved(),
				Subscription().UnknownConditions().PhysicalSubscriber(targetDNS),
			},
			WantEvent: []corev1.Event{
				events[resultResolveFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
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
							"subscribable": map[string]interface{}{},
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, all targets resolved, subscribers modified",
			InitialState: []runtime.Object{
				Subscription(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, all targets resolved, subscribers modified -- nil reply",
			InitialState: []runtime.Object{
				Subscription().NilReply(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().NilReply().ReferencesResolved().PhysicalSubscriber(targetDNS),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, all targets resolved, subscribers modified -- empty but non-nil reply",
			InitialState: []runtime.Object{
				Subscription().EmptyNonNilReply(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().ReferencesResolved().PhysicalSubscriber(targetDNS).EmptyNonNilReply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, target points to the legacy targetable interface",
			InitialState: []runtime.Object{
				Subscription().EmptyNonNilReply(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().ReferencesResolved().PhysicalSubscriber(targetDNS).EmptyNonNilReply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"domainInternal": targetDNS,
						},
					},
				},
			},
		}, {
			Name: "old subscription: updates status, removing the no longer present Subscriber",
			InitialState: []runtime.Object{
				// This will have no Subscriber in the spec, but will have one in the status.
				Subscription().NilSubscriber().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().NilSubscriber().ReferencesResolved().Reply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "old subscription: updates status, removing the no longer present reply",
			InitialState: []runtime.Object{
				// This will have no Reply in the spec, but will have one in the status.
				Subscription().NilReply().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().NilReply().ReferencesResolved().PhysicalSubscriber(targetDNS),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"domainInternal": targetDNS,
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, all targets resolved, subscribers modified -- nil subscriber",
			InitialState: []runtime.Object{
				Subscription().NilSubscriber(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().NilSubscriber().ReferencesResolved().Reply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription: adds status, all targets resolved, subscribers modified -- empty but non-nil subscriber",
			InitialState: []runtime.Object{
				Subscription().EmptyNonNilSubscriber(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().EmptyNonNilSubscriber().ReferencesResolved().Reply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription to non-existent K8s Service: fails with no service found",
			InitialState: []runtime.Object{
				Subscription().ToK8sService(),
			},
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().ToK8sService().UnknownConditions(),
			},
			WantErrMsg: "services \"testk8sservice\" not found",
			WantEvent: []corev1.Event{
				events[subscriberResolveFailed],
			},
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
							"subscribable": map[string]interface{}{},
						},
					},
				},
			},
		}, {
			Name: "new subscription to K8s Service: adds status, all targets resolved, subscribers modified",
			InitialState: []runtime.Object{
				Subscription().ToK8sService(),
				getK8sService(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantPresent: []runtime.Object{
				Subscription().ToK8sService().ReferencesResolved().PhysicalSubscriber(k8sServiceDNS).Reply(),
			},
			WantErrMsg: "invalid JSON document",
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		}, {
			Name: "new subscription with from channel: adds status, all targets resolved, subscribers modified",
			InitialState: []runtime.Object{
				Subscription(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantErrMsg: "invalid JSON document",
			WantPresent: []runtime.Object{
				Subscription().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
			},
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
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
				Subscription(),
				Subscription().Renamed().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
				// This subscription has a different physical From, so we should not see it in the same
				// Channel as the first two.
				Subscription().DifferentChannel(),
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
				Subscription().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
				// Unaltered because this Subscription was not reconciled.
				Subscription().Renamed().ReferencesResolved().PhysicalSubscriber(targetDNS).Reply(),
				Subscription().DifferentChannel(),
			},
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"subscribable": map[string]interface{}{},
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
							"subscribable": map[string]interface{}{},
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
							"address": map[string]interface{}{
								"hostname": targetDNS,
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
							"subscribable": map[string]interface{}{},
						},
						"status": map[string]interface{}{
							"address": map[string]interface{}{
								"hostname": sinkableDNS,
							},
						},
					},
				},
			},
		},
		{
			Name: "delete subscription with from channel: subscribers modified",
			InitialState: []runtime.Object{
				Subscription().Deleted().ChannelReady(),
			},
			// TODO: JSON patch is not working on the fake, see
			// https://github.com/kubernetes/client-go/issues/478. Marking this as expecting a specific
			// failure for now, until upstream is fixed.
			WantResult: reconcile.Result{},
			WantErrMsg: "invalid JSON document",
			WantAbsent: []runtime.Object{
				// TODO: JSON patch is not working on the fake, see
				// https://github.com/kubernetes/client-go/issues/478. The entire test is really to
				// verify the following, but can't be done because the call to Patch fails (it assumes
				// a Strategic Merge Patch, whereas we are doing a JSON Patch). so for now, comment it
				// out.
				//getNewDeletedSubscriptionWithChannelReady(),
			},
			WantPresent: []runtime.Object{
				// TODO: JSON patch is not working on the fake, see
				// https://github.com/kubernetes/client-go/issues/478. The entire test is really to
				// verify the following, but can't be done because the call to Patch fails (it assumes
				// a Strategic Merge Patch, whereas we are doing a JSON Patch). so for now, comment it
				// out.
				//getChannelWithOtherSubscription(),
			},
			WantEvent: []corev1.Event{
				events[physicalChannelSyncFailed],
			},
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
							"channelable": map[string]interface{}{
								"subscribers": []interface{}{
									map[string]interface{}{
										"subscriberURI": targetDNS,
										"replyURI":      sinkableDNS,
									},
									map[string]interface{}{
										"replyURI": otherAddressableDNS,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc.Scheme = scheme.Scheme
		c := tc.GetClient()
		dc := tc.GetDynamicClient()
		recorder := tc.GetEventRecorder()

		r := &reconciler{
			client:        c,
			dynamicClient: dc,
			restConfig:    &rest.Config{},
			recorder:      recorder,
			logger:        zap.NewNop(),
		}
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, subscriptionName)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func TestFinalizers(t *testing.T) {
	testCases := []struct {
		name     string
		original sets.String
		add      bool
		want     sets.String
	}{
		{
			name:     "empty, add",
			original: sets.NewString(),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "empty, delete",
			original: sets.NewString(),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, delete",
			original: sets.NewString(finalizerName),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, add",
			original: sets.NewString(finalizerName),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "existing two, delete",
			original: sets.NewString(finalizerName, "someother"),
			add:      false,
			want:     sets.NewString("someother"),
		}, {
			name:     "existing two, no change",
			original: sets.NewString(finalizerName, "someother"),
			add:      true,
			want:     sets.NewString(finalizerName, "someother"),
		},
	}

	for _, tc := range testCases {
		original := &eventingv1alpha1.Subscription{}
		original.Finalizers = tc.original.List()
		if tc.add {
			addFinalizer(original)
		} else {
			removeFinalizer(original)
		}
		has := sets.NewString(original.Finalizers...)
		diff := has.Difference(tc.want)
		if diff.Len() > 0 {
			t.Errorf("%q failed, diff: %+v", tc.name, diff)
		}
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

type SubscriptionBuilder struct {
	*eventingv1alpha1.Subscription
}

// Verify the Builder implements Buildable
var _ controllertesting.Buildable = &SubscriptionBuilder{}

func Subscription() *SubscriptionBuilder {
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

	return &SubscriptionBuilder{
		Subscription: subscription,
	}
}

func (s *SubscriptionBuilder) Build() runtime.Object {
	return s.Subscription
}

func (s *SubscriptionBuilder) EmptyNonNilReply() *SubscriptionBuilder {
	s.Spec.Reply = &eventingv1alpha1.ReplyStrategy{}
	return s
}

func (s *SubscriptionBuilder) NilReply() *SubscriptionBuilder {
	s.Spec.Reply = nil
	return s
}

func (s *SubscriptionBuilder) EmptyNonNilSubscriber() *SubscriptionBuilder {
	s.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{}
	return s
}

func (s *SubscriptionBuilder) NilSubscriber() *SubscriptionBuilder {
	s.Spec.Subscriber = nil
	return s
}

func (s *SubscriptionBuilder) FromSource() *SubscriptionBuilder {
	s.Spec.Channel = corev1.ObjectReference{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       sourceKind,
		Name:       sourceName,
	}
	return s
}

func (s *SubscriptionBuilder) ToK8sService() *SubscriptionBuilder {
	s.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
		Ref: &corev1.ObjectReference{
			Name:       k8sServiceName,
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
	return s
}

func (s *SubscriptionBuilder) UnknownConditions() *SubscriptionBuilder {
	s.Status.InitializeConditions()
	return s
}

func (s *SubscriptionBuilder) PhysicalSubscriber(dns string) *SubscriptionBuilder {
	s.Status.PhysicalSubscription.SubscriberURI = resolve.DomainToURL(dns)
	return s
}

func (s *SubscriptionBuilder) ReferencesResolved() *SubscriptionBuilder {
	s = s.UnknownConditions()
	s.Status.MarkReferencesResolved()
	return s
}

func (s *SubscriptionBuilder) Reply() *SubscriptionBuilder {
	s.Status.PhysicalSubscription.ReplyURI = resolve.DomainToURL(sinkableDNS)
	return s
}

func (s *SubscriptionBuilder) DifferentChannel() *SubscriptionBuilder {
	s.Name = "different-channel"
	s.UID = "different-channel-UID"
	s.Status.PhysicalSubscription.SubscriberURI = "some-other-domain"
	return s
}

func (s *SubscriptionBuilder) ChannelReady() *SubscriptionBuilder {
	s = s.ReferencesResolved()
	s.Status.MarkChannelReady()
	return s
}

func (s *SubscriptionBuilder) Deleted() *SubscriptionBuilder {
	s.ObjectMeta.DeletionTimestamp = &deletionTime
	return s
}

// Renamed renames the subscription. It is intended to be used in tests that create multiple
// Subscriptions, so that there are no naming conflicts.
func (s *SubscriptionBuilder) Renamed() *SubscriptionBuilder {
	s.Name = "renamed"
	s.UID = "renamed-UID"
	s.Status.PhysicalSubscription.SubscriberURI = ""
	s.Status.PhysicalSubscription.ReplyURI = otherAddressableDNS
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
			Subscribable: &eventingduck.Subscribable{
				Subscribers: []eventingduck.ChannelSubscriberSpec{
					{
						Ref: &corev1.ObjectReference{
							APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
							Kind:       subscriptionKind,
							Namespace:  testNS,
							Name:       subscriptionName,
							UID:        "",
						},
						SubscriberURI: targetDNS,
						ReplyURI:      sinkableDNS,
					},
					{
						Ref: &corev1.ObjectReference{
							APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
							Kind:       subscriptionKind,
							Namespace:  testNS,
							Name:       "renamed",
							UID:        "renamed-UID",
						},
						ReplyURI: otherAddressableDNS,
					},
				},
			},
		},
	}
}

func getChannelWithOtherSubscription() *eventingv1alpha1.Channel {
	return &eventingv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       channelKind,
		},
		ObjectMeta: om(testNS, fromChannelName),
		Spec: eventingv1alpha1.ChannelSpec{
			Subscribable: &eventingduck.Subscribable{
				Subscribers: []eventingduck.ChannelSubscriberSpec{
					{
						ReplyURI: otherAddressableDNS,
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
