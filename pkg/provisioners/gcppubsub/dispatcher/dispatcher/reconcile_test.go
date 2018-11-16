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

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/testcreds"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/fakepubsub"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
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

	pscData          = "pscData"
	subscriptionsKey = "subscriptionsKey"
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
			Name: "Channel deleted - subscribers",
			InitialState: []runtime.Object{
				makeDeletingChannelWithSubscribers(),
				testcreds.MakeSecretWithCreds(),
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
			Name: "GetCredential fails",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
				testcreds.MakeSecretWithInvalidCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithSubscribersAndFinalizer(),
			},
			WantErrMsg: testcreds.InvalidCredsError,
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
	}
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	for _, tc := range testCases {
		if tc.Name != "Create Topic - problem creating client" {
			//continue
		}
		c := tc.GetClient()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),

			dispatcher:          nil,
			pubSubClientCreator: fakepubsub.Creator(tc.OtherTestData[pscData]),
			defaultGcpProject:   gcpProject,
			defaultSecret:       testcreds.Secret,
			defaultSecretKey:    testcreds.SecretKey,

			subscriptionsLock: sync.Mutex{},
			subscriptions:     map[channelName]map[subscriptionName]context.CancelFunc{},
		}
		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", cName)
		}
		if tc.OtherTestData[subscriptionsKey] != nil {
			r.subscriptions = tc.OtherTestData[subscriptionsKey].(map[channelName]map[subscriptionName]context.CancelFunc)
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c))
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
	c.Spec.Subscribable = subscribers
	return c
}

func makeChannelWithSubscribersAndFinalizer() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	c.Spec.Subscribable = subscribers
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
	c.Spec.Subscribable = subscribers
	return c
}

func makeDeletingChannelWithSubscribersWithoutFinalizer() *eventingv1alpha1.Channel {
	c := makeDeletingChannelWithSubscribers()
	c.Finalizers = nil
	return c
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
