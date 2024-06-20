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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/reconciler"
	. "knative.dev/pkg/reconciler/testing"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/channel/fanout"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"

	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
)

const (
	testNS                = "test-namespace"
	imcName               = "test-imc"
	twoSubscriberPatch    = `[{"op":"add","path":"/status/subscribers","value":[{"observedGeneration":1,"ready":"True","uid":"2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1"},{"observedGeneration":2,"ready":"True","uid":"34c5aec8-deb6-11e8-9f32-f2801f1b9fd1"}]}]`
	oneSubscriberPatch    = `[{"op":"add","path":"/status/subscribers","value":[{"observedGeneration":1,"ready":"True","uid":"2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1"}]}]`
	oneSubscriberReplaced = `[{"op":"replace","path":"/status/subscribers/1/uid","value":"34c5aec8-deb6-11e8-9f32-f2801f1b9fd1"}]`
)

var (
	channelServiceAddress = duckv1.Addressable{
		URL: apis.HTTP("test-imc-kn-channel.test-namespace.svc.cluster.local"),
	}

	linear      = eventingduckv1.BackoffPolicyLinear
	exponential = eventingduckv1.BackoffPolicyExponential

	subscriber1UID        = types.UID("2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1")
	subscriber2UID        = types.UID("34c5aec8-deb6-11e8-9f32-f2801f1b9fd1")
	subscriber3UID        = types.UID("43995566-deb6-11e8-9f32-f2801f1b9fd1")
	subscriber1Generation = int64(1)
	subscriber2Generation = int64(2)
	subscriber3Generation = int64(2)

	subscriber1 = eventingduckv1.SubscriberSpec{
		UID:           subscriber1UID,
		Generation:    subscriber1Generation,
		SubscriberURI: apis.HTTP("call1"),
		ReplyURI:      apis.HTTP("sink2"),
	}
	subscriber1WithLinearRetry = eventingduckv1.SubscriberSpec{
		UID:           subscriber1UID,
		Generation:    subscriber1Generation,
		SubscriberURI: apis.HTTP("call1"),
		ReplyURI:      apis.HTTP("sink2"),
		Delivery: &eventingduckv1.DeliverySpec{
			Retry:         ptr.Int32(3),
			BackoffPolicy: &linear,
		},
	}

	subscriber2 = eventingduckv1.SubscriberSpec{
		UID:           subscriber2UID,
		Generation:    subscriber2Generation,
		SubscriberURI: apis.HTTP("call2"),
		ReplyURI:      apis.HTTP("sink2"),
	}
	subscriber3 = eventingduckv1.SubscriberSpec{
		UID:           subscriber3UID,
		Generation:    subscriber3Generation,
		SubscriberURI: apis.HTTP("call3"),
		ReplyURI:      apis.HTTP("sink2"),
	}

	subscribers = []eventingduckv1.SubscriberSpec{subscriber1, subscriber2}
)

func TestAllCases(t *testing.T) {
	backoffPolicy := eventingduckv1.BackoffPolicyLinear

	const imcKey = testNS + "/" + imcName
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
			Name: "imc not ready",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS, WithInitInMemoryChannelConditions),
			},
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
				makePatch(testNS, imcName, twoSubscriberPatch),
			},
		}, {
			Name: "with subscribers, patch fails",
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
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("patch", "inmemorychannels/status"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				makePatch(testNS, imcName, twoSubscriberPatch),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "Failed patching: inducing failure for patch inmemorychannels"),
			},
			WantErr: true,
		}, {
			Name: "with subscribers, no patch",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber1UID), subscriber1Generation),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber2UID), subscriber2Generation),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
		}, {
			Name: "with subscribers, one replaced to status",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber1UID), subscriber1Generation),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber3UID), subscriber3Generation),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				makePatch(testNS, imcName, oneSubscriberReplaced),
			},
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
								Retry:         pointer.Int32(10),
								BackoffPolicy: &backoffPolicy,
								BackoffDelay:  pointer.String("PT1S"),
							},
						},
					}),
					WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				makePatch(testNS, imcName, oneSubscriberPatch),
			},
		}, {
			Name: "subscriber with invalid delivery spec",
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
								Retry:         pointer.Int32(10),
								BackoffPolicy: &backoffPolicy,
								BackoffDelay:  pointer.String("garbage"),
							},
						},
					}),
					WithInMemoryChannelAddress(channelServiceAddress),
					WithInMemoryChannelDLSUnknown(),
					WithInMemoryChannelEventPoliciesReady()),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "failed to parse Spec.BackoffDelay: expected 'P' period mark at the start: garbage"),
			},
			WantErr: true,
		}, {
			Name: "IMC deleted",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelFinalizers(finalizerName),
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
								Retry:         pointer.Int32(10),
								BackoffPolicy: &backoffPolicy,
								BackoffDelay:  pointer.String("garbage"),
							},
						},
					}),
					WithInMemoryChannelAddress(channelServiceAddress),
					WithInMemoryChannelDeleted,
					WithInMemoryChannelDLSUnknown()),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1addr.WithDuck(ctx)
		r := &Reconciler{
			multiChannelEventHandler: newFakeMultiChannelHandler(),
			messagingClientSet:       fakeeventingclient.Get(ctx).MessagingV1(),
		}
		return inmemorychannel.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetInMemoryChannelLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{SkipStatusUpdates: true, FinalizerName: finalizerName})
	}, false, logger))
}

func TestReconciler_ReconcileKind(t *testing.T) {
	subscription1, err := fanout.SubscriberSpecToFanoutConfig(subscriber1)
	if err != nil {
		t.Error(err)
	}
	subscription2, err := fanout.SubscriberSpecToFanoutConfig(subscriber2)
	if err != nil {
		t.Error(err)
	}

	testCases := map[string]struct {
		imc        *v1.InMemoryChannel
		subs       []fanout.Subscription
		wantSubs   []fanout.Subscription
		wantResult reconciler.Event
	}{
		"with no existing subscribers, 2 added": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers(subscribers),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			wantSubs: []fanout.Subscription{{
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call1"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				},
				Namespace: testNS,
			}, {
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call2"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				},
				Namespace: testNS,
			},
			},
		},
		"with one subscriber, one added": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers(subscribers),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			subs: []fanout.Subscription{*subscription1},
			wantSubs: []fanout.Subscription{{
				Namespace: testNS,
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call1"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				}}, {
				Namespace: testNS,
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call2"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				}},
			},
		},
		"with two subscribers, none added": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers(subscribers),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			subs: []fanout.Subscription{*subscription1, *subscription2},
			wantSubs: []fanout.Subscription{{
				Namespace: testNS,
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call1"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				}}, {
				Namespace: testNS,
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call2"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				}},
			},
		},
		"with two subscribers, one removed": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers([]eventingduckv1.SubscriberSpec{subscriber1}),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			subs: []fanout.Subscription{*subscription1, *subscription2},
			wantSubs: []fanout.Subscription{
				{
					Namespace: testNS,
					Subscriber: duckv1.Addressable{
						URL: apis.HTTP("call1"),
					},
					Reply: &duckv1.Addressable{
						URL: apis.HTTP("sink2"),
					}},
			},
		},
		"with two subscribers, one removed one added": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers([]eventingduckv1.SubscriberSpec{subscriber1, subscriber3}),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			subs: []fanout.Subscription{*subscription1, *subscription2},
			wantSubs: []fanout.Subscription{
				{
					Namespace: testNS,
					Subscriber: duckv1.Addressable{
						URL: apis.HTTP("call1"),
					},
					Reply: &duckv1.Addressable{
						URL: apis.HTTP("sink2"),
					}}, {
					Namespace: testNS,
					Subscriber: duckv1.Addressable{
						URL: apis.HTTP("call3"),
					},
					Reply: &duckv1.Addressable{
						URL: apis.HTTP("sink2"),
					}},
			},
		},
		"with one subscriber, with delivery spec changed": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInitInMemoryChannelConditions,
				WithInMemoryChannelDeploymentReady(),
				WithInMemoryChannelServiceReady(),
				WithInMemoryChannelEndpointsReady(),
				WithInMemoryChannelChannelServiceReady(),
				WithInMemoryChannelSubscribers([]eventingduckv1.SubscriberSpec{subscriber1WithLinearRetry}),
				WithInMemoryChannelAddress(channelServiceAddress),
				WithInMemoryChannelDLSUnknown(),
				WithInMemoryChannelEventPoliciesReady()),
			subs: []fanout.Subscription{{
				Subscriber: duckv1.Addressable{
					URL: apis.HTTP("call1"),
				},
				Reply: &duckv1.Addressable{
					URL: apis.HTTP("sink2"),
				},
				RetryConfig: &kncloudevents.RetryConfig{RetryMax: 2, BackoffPolicy: &exponential}}},
			wantSubs: []fanout.Subscription{
				{
					Namespace: testNS,
					Subscriber: duckv1.Addressable{
						URL: apis.HTTP("call1"),
					},
					Reply: &duckv1.Addressable{
						URL: apis.HTTP("sink2"),
					},
					RetryConfig: &kncloudevents.RetryConfig{RetryMax: 3, BackoffPolicy: &linear}},
			},
		},
	}
	for n, tc := range testCases {
		ctx, _ := SetupFakeContext(t, SetUpInformerSelector)
		ctx, fakeEventingClient := fakeeventingclient.With(ctx, tc.imc)
		feature.ToContext(ctx, feature.Flags{
			feature.EvenTypeAutoCreate: feature.Disabled,
		})

		oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
		dispatcher := kncloudevents.NewDispatcher(eventingtls.ClientConfig{}, oidcTokenProvider)
		// Just run the tests once with no existing handler (creates the handler) and once
		// with an existing, so we exercise both paths at once.
		fh, err := fanout.NewFanoutEventHandler(nil, fanout.Config{}, nil, nil, nil, nil, dispatcher)
		if err != nil {
			t.Error(err)
		}
		for _, fanoutHandler := range []fanout.EventHandler{nil, fh} {
			t.Run("handler-"+n, func(t *testing.T) {
				handler := newFakeMultiChannelHandler()
				if fanoutHandler != nil {
					fanoutHandler.SetSubscriptions(context.TODO(), tc.subs)
					handler.SetChannelHandler(channelServiceAddress.URL.String(), fanoutHandler)
				}
				r := &Reconciler{
					multiChannelEventHandler: handler,
					messagingClientSet:       fakeEventingClient.MessagingV1(),
					featureStore:             feature.NewStore(logtesting.TestLogger(t)),
				}
				e := r.ReconcileKind(ctx, tc.imc)
				if e != tc.wantResult {
					t.Errorf("Results differ, want %v have %v", tc.wantResult, e)
				}
				channelHandler := handler.GetChannelHandler(channelServiceAddress.URL.Host)
				if channelHandler == nil {
					t.Fatalf("Did not get handler for %s", channelServiceAddress.URL.Host)
				}
				if diff := cmp.Diff(tc.wantSubs, channelHandler.GetSubscriptions(context.TODO()), cmpopts.IgnoreFields(kncloudevents.RetryConfig{}, "Backoff", "CheckRetry"), cmpopts.IgnoreFields(fanout.Subscription{}, "UID")); diff != "" {
					t.Error("unexpected subs (+want/-got)", diff)
				}
			})
		}
	}
}

func TestReconciler_InvalidInputs(t *testing.T) {
	testCases := map[string]struct {
		imc interface{}
	}{
		"nil": {},
		"With no address": {
			imc: NewInMemoryChannel(imcName, testNS, WithInMemoryChannelDeleted),
		},
		"With invalid type": {
			imc: &subscriber1,
		},
	}
	for n, tc := range testCases {
		ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

		oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
		dispatcher := kncloudevents.NewDispatcher(eventingtls.ClientConfig{}, oidcTokenProvider)
		fh, err := fanout.NewFanoutEventHandler(nil, fanout.Config{}, nil, nil, nil, nil, dispatcher)
		if err != nil {
			t.Error(err)
		}
		for _, fanoutHandler := range []fanout.EventHandler{nil, fh} {
			t.Run("handler-"+n, func(t *testing.T) {
				handler := newFakeMultiChannelHandler()
				if fanoutHandler != nil {
					handler.SetChannelHandler(channelServiceAddress.URL.String(), fanoutHandler)
				}
				r := &Reconciler{
					multiChannelEventHandler: handler,
				}
				r.deleteFunc(tc.imc)
			})
		}
	}
}

func TestReconciler_Deletion(t *testing.T) {
	testCases := map[string]struct {
		imc *v1.InMemoryChannel
	}{
		"With address": {
			imc: NewInMemoryChannel(imcName, testNS,
				WithInMemoryChannelDeleted,
				WithInMemoryChannelAddress(channelServiceAddress)),
		},
	}
	for n, tc := range testCases {
		ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

		oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
		dispatcher := kncloudevents.NewDispatcher(eventingtls.ClientConfig{}, oidcTokenProvider)
		fh, err := fanout.NewFanoutEventHandler(nil, fanout.Config{}, nil, nil, nil, nil, dispatcher)
		if err != nil {
			t.Error(err)
		}
		for _, fanoutHandler := range []fanout.EventHandler{nil, fh} {
			t.Run("handler-"+n, func(t *testing.T) {
				handler := newFakeMultiChannelHandler()
				if fanoutHandler != nil {
					handler.SetChannelHandler(channelServiceAddress.URL.Host, fanoutHandler)
				}
				r := &Reconciler{
					multiChannelEventHandler: handler,
				}
				r.deleteFunc(tc.imc)
				if handler.GetChannelHandler(channelServiceAddress.URL.Host) != nil {
					t.Error("Got handler")
				}
			})
		}
	}
}

func makePatch(namespace, name, patch string) clientgotesting.PatchActionImpl {
	return clientgotesting.PatchActionImpl{
		ActionImpl: clientgotesting.ActionImpl{
			Namespace: namespace,
		},
		Name:  name,
		Patch: []byte(patch),
	}
}

type fakeMultiChannelHandler struct {
	handlers map[string]fanout.EventHandler
}

func newFakeMultiChannelHandler() *fakeMultiChannelHandler {
	return &fakeMultiChannelHandler{handlers: make(map[string]fanout.EventHandler, 2)}
}

func (f *fakeMultiChannelHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {}

func (f *fakeMultiChannelHandler) SetChannelHandler(host string, handler fanout.EventHandler) {
	f.handlers[host] = handler
}

func (f *fakeMultiChannelHandler) DeleteChannelHandler(host string) {
	delete(f.handlers, host)
}

func (f *fakeMultiChannelHandler) GetChannelHandler(host string) fanout.EventHandler {
	return f.handlers[host]
}

func (f *fakeMultiChannelHandler) CountChannelHandlers() int {
	return len(f.handlers)
}
