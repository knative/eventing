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

package dispatcher

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/channel/fanout"

	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/channel"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
)

const (
	testNS                = "test-namespace"
	imcName               = "test-imc"
	channelServiceAddress = "test-imc-kn-channel.test-namespace.svc.cluster.local"
)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	subscribers := []eventingduckv1.SubscriberSpec{{
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

	backoffPolicy := eventingduckv1.BackoffPolicyLinear

	imcKey := testNS + "/" + imcName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name:    "updated configuration, no channels",
			Key:     imcKey,
			WantErr: false,
		}, {
			Name: "updated configuration, one channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, imcName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", imcName),
			},
			WantErr: false,
		}, {
			Name: "with subscribers",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, imcName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", imcName),
			},

			WantErr: false,
		}, {
			Name: "subscriber with delivery spec",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers([]eventingduckv1.SubscriberSpec{
						{
							UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
							Generation:    1,
							SubscriberURI: apis.HTTP("call1"),
							ReplyURI:      apis.HTTP("sink2"),
							Delivery: &eventingduckv1.DeliverySpec{
								DeadLetterSink: &duckv1.Destination{
									URI: apis.HTTP("www.example.com"),
								},
								Retry:         pointer.Int32Ptr(10),
								BackoffPolicy: &backoffPolicy,
								BackoffDelay:  pointer.StringPtr("PT1S"),
							},
						},
					}),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, imcName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", imcName),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			dispatcher:                 &fakeDispatcher{handler: NewFakeMultiChannelHandler()},
			eventDispatcherConfigStore: channel.NewEventDispatcherConfigStore(logger),
		}
		return inmemorychannel.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetInMemoryChannelLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{SkipStatusUpdates: true, FinalizerName: finalizerName})
	}, false, logger))
}

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

type fakeMultiChannelHandler struct {
	handlers map[string]fanout.MessageHandler
}

func NewFakeMultiChannelHandler() *fakeMultiChannelHandler {
	return &fakeMultiChannelHandler{handlers: make(map[string]fanout.MessageHandler, 1)}
}
func (f *fakeMultiChannelHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

}

func (f *fakeMultiChannelHandler) SetChannelHandler(host string, handler fanout.MessageHandler) {
	f.handlers[host] = handler
}

func (f *fakeMultiChannelHandler) DeleteChannelHandler(host string) {
	delete(f.handlers, host)
}

func (f *fakeMultiChannelHandler) GetChannelHandler(host string) fanout.MessageHandler {
	return f.handlers[host]
}

type fakeDispatcher struct {
	handler multichannelfanout.MultiChannelMessageHandler
}

func (d *fakeDispatcher) GetHandler(_ context.Context) multichannelfanout.MultiChannelMessageHandler {
	return d.handler
}

func (d *fakeDispatcher) UpdateConfig(_ context.Context, _ channel.EventDispatcherConfig, _ *multichannelfanout.Config) error {
	// TODO set error
	return nil
}
