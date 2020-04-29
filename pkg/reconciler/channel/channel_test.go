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

package channel

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1beta1/channelable"
	channelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1beta1/channel"
	"knative.dev/eventing/pkg/duck"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS      = "test-namespace"
	channelName = "test-channel"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, channelName)

	backingChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())
)

func init() {
	// Add types to scheme
	_ = v1beta1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "Channel not found",
		Key:  testKey,
	}, {
		Name: "Channel is being deleted",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithChannelDeletedV1Beta1),
		},
	}, {
		Name: "Backing Channel.Create error",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1),
		},
		WantCreates: []runtime.Object{
			createChannelCRD(testNS, channelName, false),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannelV1Beta1(channelName, testNS,
				WithInitChannelConditionsV1Beta1,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithBackingChannelFailedV1Beta1("ChannelFailure", "inducing failure for create inmemorychannels")),
		}},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "inmemorychannels"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "problem reconciling the backing channel: %v", "inducing failure for create inmemorychannels"),
		},
		WantErr: true,
	}, {
		Name: "Successful reconciliation",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1),
			NewInMemoryChannelV1Beta1(channelName, testNS,
				WithInitInMemoryChannelConditionsV1Beta1,
				WithInMemoryChannelDeploymentReadyV1Beta1(),
				WithInMemoryChannelServiceReadyV1Beta1(),
				WithInMemoryChannelEndpointsReadyV1Beta1(),
				WithInMemoryChannelChannelServiceReadyV1Beta1(),
				WithInMemoryChannelAddressV1Beta1(backingChannelHostname)),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %q", testKey),
		},
	}, {
		Name: "Already reconciled",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname)),
			NewInMemoryChannelV1Beta1(channelName, testNS,
				WithInitInMemoryChannelConditionsV1Beta1,
				WithInMemoryChannelDeploymentReadyV1Beta1(),
				WithInMemoryChannelServiceReadyV1Beta1(),
				WithInMemoryChannelEndpointsReadyV1Beta1(),
				WithInMemoryChannelChannelServiceReadyV1Beta1(),
				WithInMemoryChannelSubscribersV1Beta1(subscribers()),
				WithInMemoryChannelAddressV1Beta1(backingChannelHostname)),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %q", testKey),
		},
	}, {
		Name: "Backing channel created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname)),
		},
		WantCreates: []runtime.Object{
			createChannel(testNS, channelName, false),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %q", testKey),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithChannelNoAddressV1Beta1(),
				WithBackingChannelUnknownV1Beta1("BackingChannelNotConfigured", "BackingChannel has not yet been reconciled.")),
		}},
	}, {
		Name: "Generation Bump",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname),
				WithChannelGenerationV1Beta1(42)),
			NewInMemoryChannelV1Beta1(channelName, testNS,
				WithInitInMemoryChannelConditionsV1Beta1,
				WithInMemoryChannelDeploymentReadyV1Beta1(),
				WithInMemoryChannelServiceReadyV1Beta1(),
				WithInMemoryChannelEndpointsReadyV1Beta1(),
				WithInMemoryChannelChannelServiceReadyV1Beta1(),
				WithInMemoryChannelSubscribersV1Beta1(subscribers()),
				WithInMemoryChannelAddressV1Beta1(backingChannelHostname)),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname),
				WithChannelGenerationV1Beta1(42),
				// Updates
				WithChannelObservedGenerationV1Beta1(42)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %q", testKey),
		},
	}, {
		Name: "Updating subscribers statuses",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname)),
			NewInMemoryChannelV1Beta1(channelName, testNS,
				WithInitInMemoryChannelConditionsV1Beta1,
				WithInMemoryChannelDeploymentReadyV1Beta1(),
				WithInMemoryChannelServiceReadyV1Beta1(),
				WithInMemoryChannelEndpointsReadyV1Beta1(),
				WithInMemoryChannelChannelServiceReadyV1Beta1(),
				WithInMemoryChannelAddressV1Beta1(backingChannelHostname),
				WithInMemoryChannelSubscribersV1Beta1(subscribers()),
				WithInMemoryChannelStatusSubscribersV1Beta1(subscriberStatuses())),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannelV1Beta1(channelName, testNS,
				WithChannelTemplateV1Beta1(channelCRD()),
				WithInitChannelConditionsV1Beta1,
				WithBackingChannelObjRefV1Beta1(backingChannelObjRef()),
				WithBackingChannelReadyV1Beta1,
				WithChannelAddressV1Beta1(backingChannelHostname),
				WithChannelSubscriberStatusesV1Beta1(subscriberStatuses())),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %q", testKey),
		},
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		r := &Reconciler{
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			channelLister:      listers.GetV1Beta1MessagingChannelLister(),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
		}
		return channelreconciler.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetV1Beta1MessagingChannelLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}

func channelCRD() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1beta1",
		Kind:       "InMemoryChannel",
	}
}

func subscribers() []eventingduckv1beta1.SubscriberSpec {
	return []eventingduckv1beta1.SubscriberSpec{{
		UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		Generation:    1,
		SubscriberURI: apis.HTTP("call1"),
		ReplyURI:      apis.HTTP("sink2"),
	}, {
		UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		Generation:    2,
		SubscriberURI: apis.HTTP("call2"),
		ReplyURI:      apis.HTTP("sink2"),
	}}
}

func subscriberStatuses() []eventingduckv1beta1.SubscriberStatus {
	return []eventingduckv1beta1.SubscriberStatus{{
		UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 1,
		Ready:              "True",
	}, {
		UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 2,
		Ready:              "True",
	}}
}

func createChannelCRD(namespace, name string, ready bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1beta1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1beta1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Channel",
						"name":               name,
						"uid":                "",
					},
				},
			},
		},
	}
	if ready {
		u.Object["status"] = map[string]interface{}{
			"address": map[string]interface{}{
				"hostname": backingChannelHostname,
				"url":      fmt.Sprintf("http://%s", backingChannelHostname),
			},
		}
	}
	return u
}

func backingChannelObjRef() *duckv1.KReference {
	return &duckv1.KReference{
		APIVersion: "messaging.knative.dev/v1beta1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       channelName,
	}
}

func createChannel(namespace, name string, ready bool) *unstructured.Unstructured {
	var hostname string
	var url string
	if ready {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1beta1",
				"kind":       "InMemoryChannel",
				"metadata": map[string]interface{}{
					"creationTimestamp": nil,
					"namespace":         namespace,
					"name":              name,
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "messaging.knative.dev/v1beta1",
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               "Channel",
							"name":               name,
							"uid":                "",
						},
					},
				},
				"status": map[string]interface{}{
					"address": map[string]interface{}{
						"hostname": hostname,
						"url":      url,
					},
				},
			},
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1beta1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1beta1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Channel",
						"name":               name,
						"uid":                "",
					},
				},
			},
		},
	}
}
