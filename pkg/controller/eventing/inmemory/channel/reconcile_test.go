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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	util "github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	ccpName = "in-memory-channel"

	cNamespace = "test-namespace"
	cName      = "test-channel"
	cUID       = "test-uid"

	cmNamespace = cNamespace
	cmName      = "test-config-map"

	testErrorMessage = "test induced error"

	insertedByVerifyConfigMapData = "data inserted by verifyConfigMapData so that it can be WantPresent"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true

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
)

func init() {
	// Add types to scheme.
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
	istiov1alpha3.AddToScheme(scheme.Scheme)
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
			Name: "Channel deleted - Channel config sync fails",
			InitialState: []runtime.Object{
				makeDeletingChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: errorListingChannels(),
			},
			WantPresent: []runtime.Object{
				// Finalizer has not been removed.
				makeDeletingChannel(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel deleted - finalizer removed",
			InitialState: []runtime.Object{
				makeDeletingChannel(),
			},
			WantPresent: []runtime.Object{
				makeDeletingChannelWithoutFinalizer(),
			},
		},
		{
			Name: "Channel config sync fails - can't list Channels",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: errorListingChannels(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel config sync fails - can't get ConfigMap",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: errorGettingConfigMap(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel config sync fails - can't create ConfigMap",
			InitialState: []runtime.Object{
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: errorCreatingConfigMap(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel config sync fails - can't update ConfigMap",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: errorUpdatingConfigMap(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "K8s service get fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: errorGettingK8sService(),
			},
			WantPresent: []runtime.Object{
				makeChannelWithFinalizer(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "K8s service creation fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: errorCreatingK8sService(),
			},
			WantPresent: []runtime.Object{
				// TODO: This should have a useful error message saying that the K8s Service failed.
				makeChannelWithFinalizer(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "K8s service already exists - not owned by Channel",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sServiceNotOwnedByChannel(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
			},
		},
		{
			Name: "Virtual service get fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sService(),
				makeVirtualService(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: errorGettingVirtualService(),
			},
			WantPresent: []runtime.Object{
				// TODO: This should have a useful error message saying that the VirtualService
				// failed.
				makeChannelWithFinalizerAndAddress(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Virtual service creation fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: errorCreatingVirtualService(),
			},
			WantPresent: []runtime.Object{
				// TODO: This should have a useful error message saying that the VirtualService
				// failed.
				makeChannelWithFinalizerAndAddress(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "VirtualService already exists - not owned by Channel",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sService(),
				makeVirtualServiceNowOwnedByChannel(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
			},
		},
		{
			Name: "Channel get for update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sService(),
				makeVirtualService(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: errorOnSecondChannelGet(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
				makeK8sService(),
				makeVirtualService(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: errorUpdatingChannel(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Channel reconcile successful - Channel list follows pagination",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: (&paginatedChannelsListStruct{channels: channels}).MockLists(),
				// This is more accurate to be in WantPresent, but we need to check JSON equality,
				// not string equality, so it can't be done in WantPresent. Instead, we verify
				// during the update call, swapping out the data and WantPresent with that inserted
				// data.
				MockUpdates: verifyConfigMapData(channelsConfig),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
				makeK8sService(),
				makeVirtualService(),
				makeConfigMapWithVerifyConfigMapData(),
			},
		},
		{
			Name: "Channel reconcile successful - Channel has no subscribers",
			InitialState: []runtime.Object{
				makeChannel(),
				makeConfigMap(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: (&paginatedChannelsListStruct{channels: []eventingv1alpha1.Channel{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "high-consul",
							Name:      "duarte",
						},
						Spec: eventingv1alpha1.ChannelSpec{
							Provisioner: &corev1.ObjectReference{
								Name: ccpName,
							},
						},
					},
				}}).MockLists(),
				// This is more accurate to be in WantPresent, but we need to check JSON equality,
				// not string equality, so it can't be done in WantPresent. Instead, we verify
				// during the update call, swapping out the data and WantPresent with that inserted
				// data.
				MockUpdates: verifyConfigMapData(multichannelfanout.Config{
					ChannelConfigs: []multichannelfanout.ChannelConfig{
						{
							Namespace: "high-consul",
							Name:      "duarte",
						},
					},
				}),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
				makeK8sService(),
				makeVirtualService(),
				makeConfigMapWithVerifyConfigMapData(),
			},
		},
	}
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	for _, tc := range testCases {
		configMapKey := types.NamespacedName{
			Namespace: cmNamespace,
			Name:      cmName,
		}
		c := tc.GetClient()
		r := &reconciler{
			client:       c,
			recorder:     recorder,
			logger:       zap.NewNop(),
			configMapKey: configMapKey,
		}
		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", cName)
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
	return c
}

func makeChannelWithFinalizerAndAddress() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizer()
	c.Status.SetAddress(fmt.Sprintf("%s-channel.%s.svc.cluster.local", c.Name, c.Namespace))
	return c
}

func makeReadyChannel() *eventingv1alpha1.Channel {
	// Ready channels have the finalizer and are Addressable.
	c := makeChannelWithFinalizerAndAddress()
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

func makeConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cmNamespace,
			Name:      cmName,
		},
	}
}

func makeConfigMapWithVerifyConfigMapData() *corev1.ConfigMap {
	cm := makeConfigMap()
	cm.Data = map[string]string{}
	cm.Data[configmap.MultiChannelFanoutConfigKey] = insertedByVerifyConfigMapData
	return cm
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", cName),
			Namespace: cNamespace,
			Labels: map[string]string{
				"channel":     cName,
				"provisioner": ccpName,
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
			Ports: []corev1.ServicePort{
				{
					Name: util.PortName,
					Port: util.PortNumber,
				},
			},
		},
	}
}

func makeK8sServiceNotOwnedByChannel() *corev1.Service {
	svc := makeK8sService()
	svc.OwnerReferences = nil
	return svc
}

func makeVirtualService() *istiov1alpha3.VirtualService {
	return &istiov1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: istiov1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", cName),
			Namespace: cNamespace,
			Labels: map[string]string{
				"channel":     cName,
				"provisioner": ccpName,
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
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				fmt.Sprintf("%s-channel.%s.svc.cluster.local", cName, cNamespace),
				fmt.Sprintf("%s.%s.channels.cluster.local", cName, cNamespace),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: fmt.Sprintf("%s.%s.channels.cluster.local", cName, cNamespace),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: "in-memory-channel-dispatcher.knative-eventing.svc.cluster.local",
						Port: istiov1alpha3.PortSelector{
							Number: util.PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func makeVirtualServiceNowOwnedByChannel() *istiov1alpha3.VirtualService {
	vs := makeVirtualService()
	vs.OwnerReferences = nil
	return vs
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

func errorGettingConfigMap() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.ConfigMap); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

func errorGettingK8sService() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.Service); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

func errorGettingVirtualService() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*istiov1alpha3.VirtualService); ok {
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

func errorCreatingConfigMap() []controllertesting.MockCreate {
	return []controllertesting.MockCreate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.ConfigMap); ok {
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

func errorCreatingVirtualService() []controllertesting.MockCreate {
	return []controllertesting.MockCreate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*istiov1alpha3.VirtualService); ok {
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

func errorUpdatingConfigMap() []controllertesting.MockUpdate {
	return []controllertesting.MockUpdate{
		func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*corev1.ConfigMap); ok {
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

func verifyConfigMapData(expected multichannelfanout.Config) []controllertesting.MockUpdate {
	return []controllertesting.MockUpdate{
		func(innerClient client.Client, ctx context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
			if cm, ok := obj.(*corev1.ConfigMap); ok {
				s := cm.Data[configmap.MultiChannelFanoutConfigKey]
				c := multichannelfanout.Config{}
				err := json.Unmarshal([]byte(s), &c)
				if err != nil {
					return controllertesting.Handled,
						fmt.Errorf("test is unable to unmarshal ConfigMap data: %v", err)
				}
				if diff := cmp.Diff(c, expected); diff != "" {
					return controllertesting.Handled,
						fmt.Errorf("test got unwanted ChannelsConfig (-want +got) %s", diff)
				}
				// Verified it is correct, now so that we can verify this actually occurred, swap
				// out the data with a known value for later comparison.
				cm.Data[configmap.MultiChannelFanoutConfigKey] = insertedByVerifyConfigMapData
				return controllertesting.Handled, innerClient.Update(ctx, obj)
			}
			return controllertesting.Unhandled, nil
		},
	}
}
