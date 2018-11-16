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

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/testcreds"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/fakepubsub"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	util "github.com/knative/eventing/pkg/provisioners"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
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

	pscData = "pscData"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true

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
				makeChannel(),
				testcreds.MakeSecretWithInvalidCreds(),
			},
			WantPresent: []runtime.Object{
				makeChannel(),
			},
			WantErrMsg: testcreds.InvalidCredsError,
		},
		{
			Name: "K8s service get fails",
			InitialState: []runtime.Object{
				makeChannel(),
				testcreds.MakeSecretWithCreds(),
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
				testcreds.MakeSecretWithCreds(),
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
				makeK8sServiceNotOwnedByChannel(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
			},
		},
		{
			Name: "Virtual service get fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
				testcreds.MakeSecretWithCreds(),
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
				makeK8sService(),
				testcreds.MakeSecretWithCreds(),
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
				makeK8sService(),
				makeVirtualServiceNotOwnedByChannel(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
			},
		},
		{
			Name: "Create Topic - problem creating client",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
				testcreds.MakeSecretWithCreds(),
			},
			OtherTestData: map[string]interface{}{
				pscData: fakepubsub.CreatorData{
					ClientCreateErr: errors.New(testErrorMessage),
				},
			},
			WantErrMsg: testErrorMessage,
			WantPresent: []runtime.Object{
				makeChannelWithFinalizerAndAddress(),
			},
		},
		{
			Name: "Create Topic - problem checking existence",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
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
				makeChannelWithFinalizerAndAddress(),
			},
		},
		{
			Name: "Create Topic - topic already exists",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
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
				makeReadyChannel(),
			},
		},
		{
			Name: "Create Topic - error creating topic",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
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
				makeChannelWithFinalizerAndAddress(),
			},
		},
		{
			Name: "Create Topic - topic create succeeds",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannel(),
			},
		},
		{
			Name: "Create Subscriptions - problem checking exists",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
				makeK8sService(),
				makeVirtualService(),
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
				makeChannelWithSubscribersAndFinalizerAndAddress(),
			},
		},
		{
			Name: "Create Subscriptions - already exists",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
				makeK8sService(),
				makeVirtualService(),
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
		},
		{
			Name: "Create Subscriptions - create fails",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
				makeK8sService(),
				makeVirtualService(),
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
				makeChannelWithSubscribersAndFinalizerAndAddress(),
			},
		},
		{
			Name: "Create Subscriptions - create succeeds",
			InitialState: []runtime.Object{
				makeChannelWithSubscribers(),
				makeK8sService(),
				makeVirtualService(),
				testcreds.MakeSecretWithCreds(),
			},
			WantPresent: []runtime.Object{
				makeReadyChannelWithSubscribers(),
			},
		},
		{
			Name: "Channel get for update fails",
			InitialState: []runtime.Object{
				makeChannel(),
				makeK8sService(),
				makeVirtualService(),
				testcreds.MakeSecretWithCreds(),
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
				makeK8sService(),
				makeVirtualService(),
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

			pubSubClientCreator: fakepubsub.Creator(tc.OtherTestData[pscData]),
			defaultGcpProject:   gcpProject,
			defaultSecret:       testcreds.Secret,
			defaultSecretKey:    testcreds.SecretKey,
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

func makeChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Spec.Subscribable = subscribers
	return c
}

func makeChannelWithSubscribersAndFinalizerAndAddress() *eventingv1alpha1.Channel {
	c := makeChannelWithFinalizerAndAddress()
	c.Spec.Subscribable = subscribers
	return c
}

func makeChannelWithFinalizer() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Finalizers = []string{finalizerName}
	return c
}

func makeReadyChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeReadyChannel()
	c.Spec.Subscribable = subscribers
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
						Host: "in-memory-channel-clusterbus.knative-eventing.svc.cluster.local",
						Port: istiov1alpha3.PortSelector{
							Number: util.PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func makeVirtualServiceNotOwnedByChannel() *istiov1alpha3.VirtualService {
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
