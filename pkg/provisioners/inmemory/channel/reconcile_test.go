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

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/reconciler/names"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/utils"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/system"
	_ "github.com/knative/pkg/system/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	ccpName      = "in-memory-channel"
	asyncCCPName = "in-memory"

	cNamespace = "test-namespace"
	cName      = "test-channel"
	cUID       = "test-uid"

	cmNamespace = cNamespace
	cmName      = "test-config-map"

	testErrorMessage = "test induced error"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true

	// serviceAddress is the address of the K8s Service. It uses a GeneratedName and the fake client
	// does not fill in Name, so the name is the empty string.
	serviceAddress = fmt.Sprintf("%s.%s.svc.%s", "", cNamespace, utils.GetClusterDomainName())

	// channelsConfig and channels are linked together. A change to one, will likely require a
	// change to the other. channelsConfig is the serialized config of channels for everything
	// provisioned by the in-memory-provisioner.
	channelsConfig = multichannelfanout.Config{
		ChannelConfigs: []multichannelfanout.ChannelConfig{
			{
				Namespace: cNamespace,
				Name:      "c1",
				FanoutConfig: fanout.Config{
					Subscriptions: []eventingduck.ChannelSubscriberSpec{
						{
							SubscriberURI: "foo",
						},
						{
							ReplyURI: "bar",
						},
						{
							SubscriberURI: "baz",
							ReplyURI:      "qux",
						},
					},
				},
			},
			{
				Namespace: cNamespace,
				Name:      "c3",
				FanoutConfig: fanout.Config{
					Subscriptions: []eventingduck.ChannelSubscriberSpec{
						{
							SubscriberURI: "steve",
						},
					},
				},
			},
		},
	}

	channels = []eventingv1alpha1.Channel{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cNamespace,
				Name:      "c1",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Channel",
			},
			Spec: eventingv1alpha1.ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: ccpName,
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{
						{
							SubscriberURI: "foo",
						},
						{
							ReplyURI: "bar",
						},
						{
							SubscriberURI: "baz",
							ReplyURI:      "qux",
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cNamespace,
				Name:      "c2",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Channel",
			},
			Spec: eventingv1alpha1.ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "some-other-provisioner",
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{
						{
							SubscriberURI: "anything",
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cNamespace,
				Name:      "c3",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Channel",
			},
			Spec: eventingv1alpha1.ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: ccpName,
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{
						{
							SubscriberURI: "steve",
						},
					},
				},
			},
		},
	}

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		channelReconciled:         {Reason: channelReconciled, Type: corev1.EventTypeNormal},
		channelUpdateStatusFailed: {Reason: channelUpdateStatusFailed, Type: corev1.EventTypeWarning},
		k8sServiceCreateFailed:    {Reason: k8sServiceCreateFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme.
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = istiov1alpha3.AddToScheme(scheme.Scheme)
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
			Name: "Channel has finalizer (to test back compat with version <= 0.5, when finalizers were added",
			InitialState: []runtime.Object{
				makeChannelWithFinalizer(),
			},
			WantPresent: []runtime.Object{
				makeChannel(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
			WantResult: reconcile.Result{Requeue: true},
		},
		{
			Name: "K8s service get fails",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: errorListingK8sService(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[k8sServiceCreateFailed],
			},
		},
		{
			Name: "K8s service creation fails",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: errorCreatingK8sService(),
			},
			// TODO: This should have a useful error message saying that the K8s Service failed.
			WantPresent: []runtime.Object{},
			WantErrMsg:  testErrorMessage,
			WantEvent: []corev1.Event{
				events[k8sServiceCreateFailed],
			},
		},
		{
			Name: "Channel status update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: errorUpdatingChannelStatus(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[channelReconciled], events[channelUpdateStatusFailed],
			},
		},
		{
			Name: "Channel reconcile successful - Async channel",
			InitialState: []runtime.Object{
				makeChannel("in-memory"),
			},
			Mocks: controllertesting.Mocks{},
			WantPresent: []runtime.Object{
				makeK8sService("in-memory"),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		},
		{
			Name: "Channel reconcile successful - Non Async channel",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{},
			WantPresent: []runtime.Object{
				makeK8sService(),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
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
		}
		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", cName)
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeChannel(pn ...string) *eventingv1alpha1.Channel {
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
				Name: getProvisionerName(pn),
			},
		},
	}
	c.Status.InitializeConditions()
	return c
}

// getProvisionerName returns either default provisioner name defined by ccpName variable
// or, if specified, a custom provisioner name.
func getProvisionerName(pn []string) string {
	provisionerName := ccpName
	if len(pn) != 0 {
		provisionerName = pn[0]
	}
	return provisionerName
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

func makeChannelWithFinalizer() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Finalizers = []string{finalizerName}
	return c
}

func makeK8sService(pn ...string) *corev1.Service {
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
				util.EventingProvisionerLabel:    getProvisionerName(pn),
				util.OldEventingProvisionerLabel: getProvisionerName(pn),
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
			ExternalName: names.ServiceHostName(fmt.Sprintf("%s-dispatcher", getProvisionerName(pn)), system.Namespace()),
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

func errorListingChannels() []controllertesting.MockList {
	return []controllertesting.MockList{
		func(client.Client, context.Context, *client.ListOptions, runtime.Object) (controllertesting.MockHandled, error) {
			return controllertesting.Handled, errors.New(testErrorMessage)
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

type paginatedChannelsListStruct struct {
	channels []eventingv1alpha1.Channel
}

func (p *paginatedChannelsListStruct) MockLists() []controllertesting.MockList {
	return []controllertesting.MockList{
		func(_ client.Client, _ context.Context, _ *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
			if l, ok := list.(*eventingv1alpha1.ChannelList); ok {

				if len(p.channels) > 0 {
					c := p.channels[0]
					p.channels = p.channels[1:]
					l.Continue = "yes"
					l.Items = []eventingv1alpha1.Channel{
						c,
					}
				}
				return controllertesting.Handled, nil
			}
			return controllertesting.Unhandled, nil
		},
	}
}
