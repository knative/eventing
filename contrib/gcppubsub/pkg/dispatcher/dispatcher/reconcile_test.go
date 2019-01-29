/*
Copyright 2018 The Knative Authors

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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/knative/eventing/contrib/gcppubsub/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/testcreds"

	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/fakepubsub"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	ccpName = "gcp-pubsub"

	cNamespace = "test-namespace"
	cName      = "test-channel"
	cUID       = "test-uid"

	testErrorMessage = "test induced error"

	gcpProject = "gcp-project"

	pscData             = "pscData"
	reconcileChan       = "reconcileChan"
	shouldBeCanceled    = "shouldBeCanceled"
	shouldNotBeCanceled = "shouldNotBeCanceled"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	subscribers = &v1alpha1.Subscribable{
		Subscribers: []v1alpha1.ChannelSubscriberSpec{
			{
				Ref: &corev1.ObjectReference{
					Name: "sub-name",
					UID:  "sub-uid",
				},
			},
			{
				Ref: &corev1.ObjectReference{
					Name: "sub-2-name",
					UID:  "sub-2-uid",
				},
			},
		},
	}
)

func init() {
	// Add types to scheme.
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
}

func TestInjectClient(t *testing.T) {
	r := &reconciler{}
	orig := r.client
	n := fake.NewFakeClient()
	if orig == n {
		t.Errorf("Original and new clients are identical: %v", orig)
	}
	err := r.InjectClient(n)
	if err != nil {
		t.Errorf("Unexpected error injecting the client: %v", err)
	}
	if n != r.client {
		t.Errorf("Unexpected client. Expected: '%v'. Actual: '%v'", n, r.client)
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "Channel not found",
		},
		{
			Name: "Error getting Channel",
			Mocks: controllertesting.Mocks{
				MockGets: errorGettingChannel(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel not reconciled - nil provisioner",
			InitialState: []runtime.Object{
				makeChannelNilProvisioner(),
			},
		},
		{
			Name: "Channel not reconciled - nil ref",
			InitialState: []runtime.Object{
				makeChannelNilProvisioner(),
			},
		},
		{
			Name: "Channel not reconciled - namespace",
			InitialState: []runtime.Object{
				makeChannelWithWrongProvisionerNamespace(),
			},
		},
		{
			Name: "Channel not reconciled - name",
			InitialState: []runtime.Object{
				makeChannelWithWrongProvisionerName(),
			},
		},
		{
			Name: "Unable to read Internal Status",
			InitialState: []runtime.Object{
				makeChannelWithBadInternalStatus(),
			},
			WantErrMsg: "json: cannot unmarshal number into Go struct field GcpPubSubChannelStatus.secretKey of type string",
		},
		{
			Name: "Empty status.internal",
			InitialState: []runtime.Object{
				makeChannelWithBlankInternalStatus(),
			},
			WantErrMsg: "status.internal is blank",
		},
		{
			Name: "Channel deleted - subscribers",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				shouldBeCanceled: map[channelName]subscriptionName{
					key(makeChannel()): {
						Namespace: cNamespace,
						Name:      "sub-name",
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribersWithoutFinalizer(),
			},
		},
		{
			Name: "Channel deleted - finalizer removed",
			InitialState: []runtime.Object{
				makeDeletingChannel(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithoutFinalizer(),
			},
		},
		{
			Name: "Finalizer added",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
			},
			WantResult: reconcile.Result{
				Requeue: true,
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
		},
		{
			Name: "GetCredential fails",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
				testcreds.MakeSecretWithInvalidCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
			WantErrMsg: testcreds.InvalidCredsError,
		},
		{
			Name: "Channel update fails - cannot create PubSub client",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientCreateErr: errors.New(testErrorMessage),
				},
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Receive errors",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				reconcileChan: make(chan event.GenericEvent),
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ReceiveErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			AdditionalVerification: []func(*testing.T, *controllertesting.TestCase){
				func(t *testing.T, tc *controllertesting.TestCase) {
					select {
					case e := <-tc.OtherTestData[reconcileChan].(chan event.GenericEvent):
						if e.Meta.GetNamespace() != cNamespace || e.Meta.GetName() != cName {
							t.Errorf("Unexpected reconcileChan message: %v", e)
						}
					case <-time.After(time.Second):
						t.Error("Timed out waiting for the reconcileChan to get the Channel")
					}
				},
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
		},
		{
			Name: "PubSub Subscription.Receive already running",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ReceiveErr: errors.New(testErrorMessage),
						},
					},
				},
				shouldNotBeCanceled: map[channelName]subscriptionName{
					key(makeChannel()): {Namespace: subscribers.Subscribers[0].Ref.Namespace, Name: subscribers.Subscribers[0].Ref.Name},
				},
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
		},
		{
			Name: "Delete old Subscriptions",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ReceiveErr: errors.New(testErrorMessage),
						},
					},
				},
				shouldBeCanceled: map[channelName]subscriptionName{
					key(makeChannel()): {Namespace: cNamespace, Name: "old-sub"},
				},
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
		},
		{
			Name: "Delete all old Subscriptions",
			InitialState: []runtime.Object{
				makeChannelWithFinalizer(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ReceiveErr: errors.New(testErrorMessage),
						},
					},
				},
				shouldBeCanceled: map[channelName]subscriptionName{
					key(makeChannel()): {Namespace: cNamespace, Name: "old-sub"},
				},
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizer(),
			},
		},
		{
			Name: "Channel update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: errorUpdatingChannel(),
			},
			WantErrMsg: testErrorMessage,
		},
		// Note - we do not test update status since this dispatcher only adds
		// finalizers to the channel
	}

	for _, tc := range testCases {
		c := tc.GetClient()
		recorder := tc.GetEventRecorder()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),

			dispatcher:          nil,
			pubSubClientCreator: fakepubsub.Creator(tc.OtherTestData[pscData]),

			subscriptionsLock: sync.Mutex{},
			subscriptions:     map[channelName]map[subscriptionName]context.CancelFunc{},
		}
		if tc.OtherTestData[reconcileChan] != nil {
			r.reconcileChan = tc.OtherTestData[reconcileChan].(chan event.GenericEvent)
		} else {
			r.reconcileChan = make(chan event.GenericEvent)
		}

		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", cName)
		}
		cc := &cancelChecker{
			shouldCancel:         map[channelAndSubName]bool{},
			cancelledIncorrectly: map[channelAndSubName]bool{},
		}
		if tc.OtherTestData[shouldBeCanceled] != nil {
			for c, s := range tc.OtherTestData[shouldBeCanceled].(map[channelName]subscriptionName) {
				if r.subscriptions[c] == nil {
					r.subscriptions[c] = map[subscriptionName]context.CancelFunc{}
				}
				r.subscriptions[c][s] = cc.wantCancel(c, s)
			}
		}
		if tc.OtherTestData[shouldNotBeCanceled] != nil {
			for c, s := range tc.OtherTestData[shouldNotBeCanceled].(map[channelName]subscriptionName) {
				if r.subscriptions[c] == nil {
					r.subscriptions[c] = map[subscriptionName]context.CancelFunc{}
				}
				r.subscriptions[c][s] = cc.wantNotCancel(c, s)
			}
		}
		tc.AdditionalVerification = append(tc.AdditionalVerification, cc.verify)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func TestReceiveFunc(t *testing.T) {
	testCases := map[string]struct {
		ack           bool
		dispatcherErr error
	}{
		"dispatch error": {
			ack:           false,
			dispatcherErr: errors.New(testErrorMessage),
		},
		"dispatch success": {
			ack: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			sub := util.GcpPubSubSubscriptionStatus{
				SubscriberURI: "subscriber-uri",
				ReplyURI:      "reply-uri",
				Subscription:  "foo",
			}
			defaults := provisioners.DispatchDefaults{
				Namespace: cNamespace,
			}
			rf := receiveFunc(zap.NewNop().Sugar(), sub, defaults, &fakeDispatcher{err: tc.dispatcherErr})
			msg := fakepubsub.Message{}
			rf(context.TODO(), &msg)

			if msg.MessageData.Ack && msg.MessageData.Nack {
				t.Error("Message both Acked and Nacked")
			}
			if tc.ack {
				if !msg.MessageData.Ack {
					t.Error("Message should have been Acked. It wasn't.")
				}
			} else {
				if !msg.MessageData.Nack {
					t.Error("Message should have been Nacked. It wasn't.")
				}
			}
		})
	}
}

func makeChannel() *eventingv1alpha1.Channel {
	c := &eventingv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cNamespace,
			Name:      cName,
			UID:       cUID,
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name: ccpName,
			},
		},
	}
	c.Status.InitializeConditions()
	c.Status.SetAddress(fmt.Sprintf("%s-channel.%s.svc.cluster.local", c.Name, c.Namespace))
	c.Status.MarkProvisioned()
	pcs := &util.GcpPubSubChannelStatus{
		GCPProject: gcpProject,
		Secret:     testcreds.Secret,
		SecretKey:  testcreds.SecretKey,
	}
	if err := util.SetInternalStatus(context.Background(), c, pcs); err != nil {
		panic(err)
	}
	return c
}

func makeChannelNilProvisioner() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Spec.Provisioner = nil
	return c
}

func makeChannelWithWrongProvisionerNamespace() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Spec.Provisioner.Namespace = "wrong-namespace"
	return c
}

func makeChannelWithWrongProvisionerName() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Spec.Provisioner.Name = "wrong-name"
	return c
}

func makeChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeChannel()
	addSubscribers(c, subscribers)
	return c
}

func makeChannelWithSubscribersAndFinalizer() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	addSubscribers(c, subscribers)
	return c
}

func makeChannelWithFinalizer() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Finalizers = []string{finalizerName}
	return c
}

func makeDeletingChannel() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	c.DeletionTimestamp = &deletionTime
	return c
}

func makeDeletingChannelWithoutFinalizer() *eventingv1alpha1.Channel {
	c := makeDeletingChannel()
	c.Finalizers = nil
	return c
}

func makeDeletingChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeDeletingChannel()
	addSubscribers(c, subscribers)
	return c
}

func makeDeletingChannelWithSubscribersWithoutFinalizer() *eventingv1alpha1.Channel {
	c := makeDeletingChannelWithSubscribers()
	c.Finalizers = nil
	return c
}

func makeChannelWithBadInternalStatus() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Status.Internal = &runtime.RawExtension{
		// SecretKey must be a string, not an integer, so this will fail during json.Unmarshal.
		Raw: []byte(`{"secretKey": 123}`),
	}
	return c
}

func makeChannelWithBlankInternalStatus() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Status.Internal = nil
	return c
}

func addSubscribers(c *eventingv1alpha1.Channel, subscribable *v1alpha1.Subscribable) {
	c.Spec.Subscribable = subscribable
	pcs, err := util.GetInternalStatus(c)
	if err != nil {
		panic(err)
	}
	for _, sub := range subscribable.Subscribers {
		pcs.Subscriptions = append(pcs.Subscriptions, util.GcpPubSubSubscriptionStatus{
			Ref:           sub.Ref,
			ReplyURI:      sub.ReplyURI,
			SubscriberURI: sub.SubscriberURI,
		})
	}
	err = util.SetInternalStatus(context.Background(), c, pcs)
	if err != nil {
		panic(err)
	}
}

func errorGettingChannel() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.Channel); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

func errorUpdatingChannel() []controllertesting.MockUpdate {
	return []controllertesting.MockUpdate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.Channel); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

type channelAndSubName struct {
	c channelName
	s subscriptionName
}

type cancelChecker struct {
	shouldCancel         map[channelAndSubName]bool
	cancelledIncorrectly map[channelAndSubName]bool
}

func (cc *cancelChecker) wantCancel(c channelName, s subscriptionName) context.CancelFunc {
	n := channelAndSubName{
		c: c,
		s: s,
	}
	cc.shouldCancel[n] = false
	return func() {
		delete(cc.shouldCancel, n)
	}
}

func (cc *cancelChecker) wantNotCancel(c channelName, s subscriptionName) context.CancelFunc {
	return func() {
		n := channelAndSubName{
			c: c,
			s: s,
		}
		cc.cancelledIncorrectly[n] = false
	}
}

func (cc *cancelChecker) verify(t *testing.T, _ *controllertesting.TestCase) {
	for n := range cc.shouldCancel {
		t.Errorf("Expected to be canceled, but wasn't: %v", n)
	}
	for n := range cc.cancelledIncorrectly {
		t.Errorf("Expected not to be canceled, but was: %v", n)
	}
}

type fakeDispatcher struct {
	err error
}

func (d *fakeDispatcher) DispatchMessage(_ *provisioners.Message, _, _ string, _ provisioners.DispatchDefaults) error {
	return d.err
}
