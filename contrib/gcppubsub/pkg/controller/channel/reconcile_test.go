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

package channel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pubsubutil "github.com/knative/eventing/contrib/gcppubsub/pkg/util"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/fakepubsub"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/testcreds"
	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/reconciler/names"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ccpName = "gcp-pubsub"

	cNamespace = "test-namespace"
	cName      = "test-channel"
	cUID       = "test-uid"
	topicName  = "knative-eventing-channel_test-channel_test-uid"

	testErrorMessage = "test induced error"

	gcpProject = "gcp-project"

	pscData = "pscData"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true

	subscribers = &v1alpha1.Subscribable{
		Subscribers: []v1alpha1.SubscriberSpec{
			{
				DeprecatedRef: &corev1.ObjectReference{
					Name: "sub-name",
					UID:  "sub-uid",
				},
				UID: "sub-uid",
			},
			{
				DeprecatedRef: &corev1.ObjectReference{
					Name: "sub-2-name",
					UID:  "sub-2-uid",
				},
				UID: "sub-2-uid",
			},
		},
	}

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		channelReconciled:         {Reason: channelReconciled, Type: corev1.EventTypeNormal},
		channelUpdateStatusFailed: {Reason: channelUpdateStatusFailed, Type: corev1.EventTypeWarning},
		channelReadStatusFailed:   {Reason: channelReadStatusFailed, Type: corev1.EventTypeWarning},
		gcpCredentialsReadFailed:  {Reason: gcpCredentialsReadFailed, Type: corev1.EventTypeWarning},
		gcpResourcesPlanFailed:    {Reason: gcpResourcesPlanFailed, Type: corev1.EventTypeWarning},
		gcpResourcesPersistFailed: {Reason: gcpResourcesPersistFailed, Type: corev1.EventTypeWarning},
		k8sServiceCreateFailed:    {Reason: k8sServiceCreateFailed, Type: corev1.EventTypeWarning},
		topicCreateFailed:         {Reason: topicCreateFailed, Type: corev1.EventTypeWarning},
		topicDeleteFailed:         {Reason: topicDeleteFailed, Type: corev1.EventTypeWarning},
		subscriptionSyncFailed:    {Reason: subscriptionSyncFailed, Type: corev1.EventTypeWarning},
		subscriptionDeleteFailed:  {Reason: subscriptionDeleteFailed, Type: corev1.EventTypeWarning},
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
			Name: "Channel deleted - problem creating client to delete subscriptions",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientCreateErr: errors.New(testErrorMessage),
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[subscriptionSyncFailed],
			},
		},
		{
			Name: "Channel deleted - problem checking subscription existence",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ExistsErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[subscriptionSyncFailed],
			},
		},
		{
			Name: "Channel deleted - subscription does not exist",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							Exists: false,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribersWithoutFinalizer(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel deleted - subscription deletion fails",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							Exists:    true,
							DeleteErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[subscriptionSyncFailed],
			},
		},
		{
			Name: "Channel deleted - subscription deletion succeeds",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							Exists: true,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribersWithoutFinalizer(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel deleted - problem checking topic existence",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							ExistsErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[topicDeleteFailed],
			},
		},
		{
			Name: "Channel deleted - No status.internal",
			InitialState: []runtime.Object{
				makeDeletingChannelWithoutPCS(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							ExistsErr: errors.New("should not be seen"),
							DeleteErr: errors.New("should not be seen"),
						},
						SubscriptionData: fakepubsub.SubscriptionData{
							ExistsErr: errors.New("should not be seen"),
							DeleteErr: errors.New("should not be seen"),
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithoutFinalizerOrPCS(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel deleted - topic does not exist",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							Exists: false,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribersWithoutFinalizer(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel deleted - topic deletion fails",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							Exists:    true,
							DeleteErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[topicDeleteFailed],
			},
		},
		{
			Name: "Channel deleted - topic deletion succeeds",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							Exists: true,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithSubscribersWithoutFinalizer(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
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
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Finalizer added",
			InitialState: []runtime.Object{
				makeChannel(),
				testcreds.MakeSecretWithCreds(),
			},
			WantResult: reconcile.Result{
				Requeue: true,
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizer(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "GetCredential fails",
			InitialState: []runtime.Object{
				makeChannelWithFinalizer(),
				testcreds.MakeSecretWithInvalidCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizer(),
			},
			WantErrMsg: testcreds.InvalidCredsError,
			WantEvent: []corev1.Event{
				events[gcpCredentialsReadFailed],
			},
		},
		{
			Name: "Error reading status.internal",
			InitialState: []runtime.Object{
				makeChannelWithBadInternalStatus(),
			},
			WantErrMsg: "json: cannot unmarshal number into Go struct field GcpPubSubChannelStatus.topic of type string",
			WantEvent: []corev1.Event{
				events[channelReadStatusFailed],
			},
		},
		{
			Name: "K8s service get fails",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: errorListingK8sService(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[k8sServiceCreateFailed],
			},
		},
		{
			Name: "K8s service creation fails",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: errorCreatingK8sService(),
			},
			WantPresent: []runtime.Object{
				// TODO: This should have a useful error message saying that the K8s Service failed.
				makeChannelWithFinalizerAndPCS(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[k8sServiceCreateFailed],
			},
		},
		{
			Name: "Error planning - subscriber missing UID",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndSubscriberWithoutUID(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndSubscriberWithoutUID(),
			},
			WantErrMsg: "empty reference UID: {nil  0 http://foo/ }",
			WantEvent: []corev1.Event{
				events[gcpResourcesPlanFailed],
			},
		},
		{
			Name: "Persist plan",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPossiblyOutdatedPlan(true),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndPossiblyOutdatedPlan(false),
			},
			WantResult: reconcile.Result{
				Requeue: true,
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Create Topic - problem creating client",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientCreateErr: errors.New(testErrorMessage),
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndPCSAndAddress(),
			},
			WantEvent: []corev1.Event{
				events[topicCreateFailed],
			},
		},
		{
			Name: "Create Topic - problem checking existence",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							ExistsErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndPCSAndAddress(),
			},
			WantEvent: []corev1.Event{
				events[topicCreateFailed],
			},
		},
		{
			Name: "Create Topic - topic already exists",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						TopicData: fakepubsub.TopicData{
							Exists: true,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeK8sService(),
				makeReadyChannel(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Create Topic - error creating topic",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						CreateTopicErr: errors.New(testErrorMessage),
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndPCSAndAddress(),
			},
			WantEvent: []corev1.Event{
				events[topicCreateFailed],
			},
		},
		{
			Name: "Create Topic - topic create succeeds",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
				makeK8sService(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Create Subscriptions - problem checking exists",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							ExistsErr: errors.New(testErrorMessage),
						},
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCSAndAddress(),
			},
			WantEvent: []corev1.Event{
				events[subscriptionSyncFailed],
			},
		},
		{
			Name: "Create Subscriptions - already exists",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						SubscriptionData: fakepubsub.SubscriptionData{
							Exists: true,
						},
					},
				},
			},
			WantPresent: []runtime.Object{
				makeReadyChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Create Subscriptions - create fails",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientData: fakepubsub.ClientData{
						CreateSubErr: errors.New(testErrorMessage),
					},
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCSAndAddress(),
			},
			WantEvent: []corev1.Event{
				events[subscriptionSyncFailed],
			},
		},
		{
			Name: "Create Subscriptions - create succeeds",
			InitialState: []runtime.Object{
				makeChannelWithSubscribersAndFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannelWithSubscribers(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel get for update fails",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: errorOnSecondChannelGet(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[channelReconciled], events[channelUpdateStatusFailed],
			},
		},
		{
			Name: "Channel update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: errorUpdatingChannel(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[channelReconciled], events[channelUpdateStatusFailed],
			},
		}, {
			Name: "Channel status update fails",
			InitialState: []runtime.Object{
				makeChannelWithFinalizerAndPCS(),
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: errorUpdatingChannelStatus(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[channelReconciled], events[channelUpdateStatusFailed],
			},
		},
	}

	for _, tc := range testCases {
		c := tc.GetClient()
		recorder := tc.GetEventRecorder()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),

			pubSubClientCreator: fakepubsub.Creator(tc.OtherTestData[pscData]),
			defaultGcpProject:   gcpProject,
			defaultSecret:       testcreds.Secret,
			defaultSecretKey:    testcreds.SecretKey,
		}
		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", cName)
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
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
	return c
}

func makeChannelWithFinalizerAndPCSAndAddress() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndPCS()
	// serviceAddress is the address of the K8s Service. It uses a GeneratedName and the fake client
	// does not fill in Name, so the name is the empty string.
	c.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf(".%s.svc.%s", c.Namespace, utils.GetClusterDomainName()),
	})
	return c
}

func makeReadyChannel() *eventingv1alpha1.Channel {
	// Ready channels have the finalizer and are Addressable.
	c := makeChannelWithFinalizerAndPCSAndAddress()
	c.Status.MarkProvisioned()
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

func makeChannelWithSubscribersAndFinalizerAndPCS() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndPCS()
	addSubscribers(c, subscribers)
	return c
}

func makeChannelWithSubscribersAndFinalizerAndPCSAndAddress() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndPCSAndAddress()
	addSubscribers(c, subscribers)
	return c
}

func makeChannelWithFinalizer() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Finalizers = []string{finalizerName}
	c.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)
	return c
}

func makeChannelWithFinalizerAndPCS() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	err := pubsubutil.SetInternalStatus(context.Background(), c, &pubsubutil.GcpPubSubChannelStatus{
		Secret:     testcreds.Secret,
		SecretKey:  testcreds.SecretKey,
		GCPProject: gcpProject,
		Topic:      topicName,
	})
	if err != nil {
		panic(err)
	}
	return c
}

func makeReadyChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeReadyChannel()
	addSubscribers(c, subscribers)
	return c
}

func makeDeletingChannel() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndPCS()
	c.DeletionTimestamp = &deletionTime
	return c
}

func makeDeletingChannelWithoutPCS() *eventingv1alpha1.Channel {
	c := makeDeletingChannel()
	c.Status.Internal = nil
	return c
}

func makeDeletingChannelWithoutFinalizer() *eventingv1alpha1.Channel {
	c := makeDeletingChannel()
	c.Finalizers = nil
	c.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)
	return c
}

func makeDeletingChannelWithoutFinalizerOrPCS() *eventingv1alpha1.Channel {
	c := makeDeletingChannelWithoutFinalizer()
	c.Status.Internal = nil
	c.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)
	return c
}

func makeDeletingChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeDeletingChannel()
	addSubscribers(c, subscribers)
	c.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)
	return c
}

func makeDeletingChannelWithSubscribersWithoutFinalizer() *eventingv1alpha1.Channel {
	c := makeDeletingChannelWithSubscribers()
	c.Finalizers = nil
	c.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)
	return c
}

func makeChannelWithBadInternalStatus() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Status.Internal = &runtime.RawExtension{
		// The topic field is a string, so this will have an error during unmarshal.
		Raw: []byte(`{"topic": 123}`),
	}
	return c
}

func makeChannelWithFinalizerAndSubscriberWithoutUID() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	c.Spec.Subscribable = &v1alpha1.Subscribable{
		Subscribers: []v1alpha1.SubscriberSpec{
			{
				UID:           "",
				SubscriberURI: "http://foo/",
			},
		},
	}
	return c
}

func makeChannelWithFinalizerAndPossiblyOutdatedPlan(outdated bool) *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndPCS()
	pcs, err := pubsubutil.GetInternalStatus(c)
	if err != nil {
		panic(err)
	}

	// Add all subs to the plan.
	var plannedSubUIDs []types.UID
	if outdated {
		// If it is outdated, then the plan does not yet contain add-sub, which is present in the
		// spec.
		plannedSubUIDs = []types.UID{"keep-sub", "delete-sub"}
	} else {
		// If it is not outdated, then it still contains delete-sub (which isn't in the spec)
		// because delete-sub needs to be retained so that it can be deleted on the subsequent
		// reconcile.
		plannedSubUIDs = []types.UID{"keep-sub", "add-sub", "delete-sub"}
	}
	for _, plannedSubUID := range plannedSubUIDs {
		sub := pubsubutil.GcpPubSubSubscriptionStatus{
			SubscriberSpec: v1alpha1.SubscriberSpec{
				DeprecatedRef: &corev1.ObjectReference{
					Name: string(plannedSubUID),
					UID:  plannedSubUID,
				},
				UID: plannedSubUID,
			},
			Subscription: "will-be-retained-in-the-plan-without-recalculation",
		}
		if plannedSubUID == "add-sub" {
			sub.Subscription = "knative-eventing-channel_add-sub_add-sub"
		}
		pcs.Subscriptions = append(pcs.Subscriptions, sub)
	}

	err = pubsubutil.SetInternalStatus(context.Background(), c, pcs)
	if err != nil {
		panic(err)
	}

	// Overwrite the spec subs.
	c.Spec.Subscribable = &v1alpha1.Subscribable{
		Subscribers: []v1alpha1.SubscriberSpec{
			{
				DeprecatedRef: &corev1.ObjectReference{
					Name: "keep-sub",
					UID:  "keep-sub",
				},
				UID: "keep-sub",
			},
			{
				DeprecatedRef: &corev1.ObjectReference{
					Name: "add-sub",
					UID:  "add-sub",
				},
				UID: "add-sub",
			},
		},
	}

	return c
}

func addSubscribers(c *eventingv1alpha1.Channel, subscribable *v1alpha1.Subscribable) {
	c.Spec.Subscribable = subscribable
	pcs, err := pubsubutil.GetInternalStatus(c)
	if err != nil {
		panic(err)
	}
	for _, sub := range subscribable.Subscribers {
		pcs.Subscriptions = append(pcs.Subscriptions, pubsubutil.GcpPubSubSubscriptionStatus{
			SubscriberSpec: v1alpha1.SubscriberSpec{
				DeprecatedRef: sub.DeprecatedRef,
				UID:           sub.UID,
				ReplyURI:      sub.ReplyURI,
				SubscriberURI: sub.SubscriberURI,
			},
			Subscription: "test-subscription-id",
		})
	}
	err = pubsubutil.SetInternalStatus(context.Background(), c, pcs)
	if err != nil {
		panic(err)
	}
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-channel-", cName),
			Namespace:    cNamespace,
			Labels: map[string]string{
				util.EventingChannelLabel:        cName,
				util.OldEventingChannelLabel:     cName,
				util.EventingProvisionerLabel:    ccpName,
				util.OldEventingProvisionerLabel: ccpName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               cName,
					UID:                cUID,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			ExternalName: names.ServiceHostName(fmt.Sprintf("%s-dispatcher", ccpName), system.Namespace()),
			Type:         corev1.ServiceTypeExternalName,
		},
	}
}

func errorOnSecondChannelGet() []controllertesting.MockGet {
	passThrough := []controllertesting.MockGet{
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			return controllertesting.Handled, innerClient.Get(ctx, key, obj)
		},
	}
	return append(passThrough, errorGettingChannel()...)
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

func errorListingK8sService() []controllertesting.MockList {
	return []controllertesting.MockList{
		func(_ client.Client, _ context.Context, _ *client.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.ServiceList); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}
func errorCreatingK8sService() []controllertesting.MockCreate {
	return []controllertesting.MockCreate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.Service); ok {
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

func errorUpdatingChannelStatus() []controllertesting.MockStatusUpdate {
	return []controllertesting.MockStatusUpdate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.Channel); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}
