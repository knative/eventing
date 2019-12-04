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
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS      = "test-namespace"
	channelName = "test-channel"
)

var (
	trueVal = true

	testKey = fmt.Sprintf("%s/%s", testNS, channelName)

	backingChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())

	channelGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}

	imcGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "InMemoryChannel",
	}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {

	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "Channel not found",
			Key:  testKey,
		},
		{
			Name: "Channel is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelDeleted),
			},
		},
		{
			Name: "Backing Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions),
			},
			WantCreates: []runtime.Object{
				createChannelCRD(testNS, channelName, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithChannelTemplate(channelCRD()),
					WithBackingChannelFailed("ChannelFailure", "inducing failure for create inmemorychannels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "inmemorychannels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileError, "Channel reconcile error: problem reconciling the backing channel: %v", "inducing failure for create inmemorychannels"),
			},
			WantErr: true,
		},
		{
			Name: "Backing Channel.Patch Subscriptions failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelSubscribers(subscribers())),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, subscribers()),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("patch", "inmemorychannels"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelSubscribers(subscribers()),
					WithBackingChannelObjRef(backingChannelObjRef()),
					WithBackingChannelFailed("ChannelFailure", "inducing failure for patch inmemorychannels")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileError, "Channel reconcile error: problem patching subscriptions in the backing channel: %v", "inducing failure for patch inmemorychannels"),
			},
			WantErr: true,
		},
		{
			Name: "Successful reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelSubscribers(subscribers())),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(backingChannelHostname)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, subscribers()),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelSubscribers(subscribers()),
					WithBackingChannelObjRef(backingChannelObjRef()),
					WithBackingChannelReady,
					WithChannelAddress(backingChannelHostname)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %s", testKey),
				Eventf(corev1.EventTypeNormal, channelReadinessChanged, "Channel %q became ready", channelName),
			},
		},
		{
			Name: "Already reconciled",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithBackingChannelObjRef(backingChannelObjRef()),
					WithChannelSubscribers(subscribers()),
					WithBackingChannelReady,
					WithChannelAddress(backingChannelHostname)),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers()),
					WithInMemoryChannelAddress(backingChannelHostname)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %s", testKey),
			},
		},
		{
			Name: "Updating subscribers statuses",
			Key:  testKey,
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithBackingChannelObjRef(backingChannelObjRef()),
					WithBackingChannelReady,
					WithChannelAddress(backingChannelHostname),
					WithChannelSubscribers(subscribers())),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(backingChannelHostname),
					WithInMemoryChannelSubscribers(subscribers()),
					WithInMemoryChannelStatusSubscribers(subscriberStatuses())),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelTemplate(channelCRD()),
					WithInitChannelConditions,
					WithChannelSubscribers(subscribers()),
					WithBackingChannelObjRef(backingChannelObjRef()),
					WithBackingChannelReady,
					WithChannelAddress(backingChannelHostname),
					WithChannelSubscriberStatuses(subscriberStatuses())),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %s", testKey),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			channelLister:      listers.GetMessagingChannelLister(),
			channelableTracker: duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
		}
	},
		false,
		logger,
	))
}

func channelCRD() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "InMemoryChannel",
	}
}

func subscribers() []eventingduckv1alpha1.SubscriberSpec {
	return []eventingduckv1alpha1.SubscriberSpec{{
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

func subscriberStatuses() []eventingduckv1alpha1.SubscriberStatus {
	return []eventingduckv1alpha1.SubscriberStatus{{
		UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 1,
		Ready:              "True",
	}, {
		UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 2,
		Ready:              "True",
	}}
}

func patchSubscribers(namespace, name string, subscribers []eventingduckv1alpha1.SubscriberSpec) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	var spec string
	if subscribers != nil {
		b, err := json.Marshal(subscribers)
		if err != nil {
			return action
		}
		ss := make([]map[string]interface{}, 0)
		err = json.Unmarshal(b, &ss)
		if err != nil {
			return action
		}
		subs, err := json.Marshal(ss)
		if err != nil {
			return action
		}
		spec = fmt.Sprintf(`{"subscribable":{"subscribers":%s}}`, subs)
	} else {
		spec = `{"subscribable":{}}`
	}

	patch := `{"spec":` + spec + `}`
	action.Patch = []byte(patch)
	return action
}

func createChannelCRD(namespace, name string, ready bool) *unstructured.Unstructured {
	unstructured := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
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
		unstructured.Object["status"] = map[string]interface{}{
			"address": map[string]interface{}{
				"hostname": backingChannelHostname,
				"url":      fmt.Sprintf("http://%s", backingChannelHostname),
			},
		}
	}
	return unstructured
}

func backingChannelObjRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       channelName,
	}
}
