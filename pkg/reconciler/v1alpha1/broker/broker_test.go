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

package broker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/knative/eventing/pkg/utils"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testNS     = "test-namespace"
	brokerName = "test-broker"

	filterImage  = "filter-image"
	filterSA     = "filter-SA"
	ingressImage = "ingress-image"
	ingressSA    = "ingress-SA"
)

var (
	trueVal = true

	channelProvisioner = &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "ClusterChannelProvisioner",
		Name:       "my-provisioner",
	}

	channelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())

	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestProvideController(t *testing.T) {
	// TODO(grantr) This needs a mock of manager.Manager. Creating a manager
	// with a fake Config fails because the Manager tries to contact the
	// apiserver.

	// cfg := &rest.Config{
	// 	Host: "http://foo:80",
	// }
	//
	// mgr, err := manager.New(cfg, manager.Options{})
	// if err != nil {
	// 	t.Fatalf("Error creating manager: %v", err)
	// }
	//
	// _, err = ProvideController(mgr)
	// if err != nil {
	// 	t.Fatalf("Error in ProvideController: %v", err)
	// }
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
			Name: "Broker not found",
		},
		{
			Name:   "Broker.Get fails",
			Scheme: scheme.Scheme,
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error getting the Broker")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the Broker",
		},
		{
			Name:   "Broker is being deleted",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeDeletingBroker(),
			},
			WantEvent: []corev1.Event{
				{
					Reason: brokerReconciled, Type: corev1.EventTypeNormal,
				},
			},
		},
		{
			Name:   "Channel.List error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := list.(*v1alpha1.ChannelList); ok {
							return controllertesting.Handled, errors.New("test error listing channels")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error listing channels",
		},
		{
			Name:   "Channel.Create error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Channel); ok {
							return controllertesting.Handled, errors.New("test error creating Channel")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating Channel",
		},
		{
			Name:   "Channel is different than expected",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeDifferentChannel(),
			},
			WantPresent: []runtime.Object{
				// This is special because the Channel is not updated, unlike most things that
				// differ from expected.
				// TODO uncomment the following line once our test framework supports searching for
				// GenerateName.
				// makeDifferentChannel(),
			},
			WantEvent: []corev1.Event{
				{
					Reason: brokerReconciled, Type: corev1.EventTypeNormal,
				},
			},
		},
		{
			Name:   "Channel is not yet Addressable",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeNonAddressableChannel(),
			},
			WantResult: reconcile.Result{RequeueAfter: time.Second},
		},
		{
			Name:   "Filter Deployment.Get error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*appsv1.Deployment); ok {
							if strings.Contains(key.Name, "filter") {
								return controllertesting.Handled, errors.New("test error getting filter Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting filter Deployment",
		},
		{
			Name:   "Filter Deployment.Create error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if d, ok := obj.(*appsv1.Deployment); ok {
							if d.Labels["eventing.knative.dev/brokerRole"] == "filter" {
								return controllertesting.Handled, errors.New("test error creating filter Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating filter Deployment",
		},
		{
			Name:   "Filter Deployment.Update error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
				makeDifferentFilterDeployment(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if d, ok := obj.(*appsv1.Deployment); ok {
							if d.Labels["eventing.knative.dev/brokerRole"] == "filter" {
								return controllertesting.Handled, errors.New("test error updating filter Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating filter Deployment",
		},
		{
			Name:   "Filter Service.Get error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*corev1.Service); ok {
							if strings.Contains(key.Name, "filter") {
								return controllertesting.Handled, errors.New("test error getting filter Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting filter Service",
		},
		{
			Name:   "Filter Service.Create error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if svc, ok := obj.(*corev1.Service); ok {
							if svc.Labels["eventing.knative.dev/brokerRole"] == "filter" {
								return controllertesting.Handled, errors.New("test error creating filter Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating filter Service",
		},
		{
			Name:   "Filter Service.Update error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
				makeDifferentFilterService(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if svc, ok := obj.(*corev1.Service); ok {
							if svc.Labels["eventing.knative.dev/brokerRole"] == "filter" {
								return controllertesting.Handled, errors.New("test error updating filter Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating filter Service",
		},
		{
			Name:   "Ingress Deployment.Get error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*appsv1.Deployment); ok {
							if strings.Contains(key.Name, "ingress") {
								return controllertesting.Handled, errors.New("test error getting ingress Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting ingress Deployment",
		},
		{
			Name:   "Ingress Deployment.Create error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if d, ok := obj.(*appsv1.Deployment); ok {
							if d.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
								return controllertesting.Handled, errors.New("test error creating ingress Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating ingress Deployment",
		},
		{
			Name:   "Ingress Deployment.Update error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
				makeDifferentIngressDeployment(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if d, ok := obj.(*appsv1.Deployment); ok {
							if d.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
								return controllertesting.Handled, errors.New("test error updating ingress Deployment")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating ingress Deployment",
		},
		{
			Name:   "Ingress Service.Get error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*corev1.Service); ok {
							if key.Name == fmt.Sprintf("%s-broker", brokerName) {
								return controllertesting.Handled, errors.New("test error getting ingress Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting ingress Service",
		},
		{
			Name:   "Ingress Service.Create error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if svc, ok := obj.(*corev1.Service); ok {
							if svc.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
								return controllertesting.Handled, errors.New("test error creating ingress Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error creating ingress Service",
		},
		{
			Name:   "Ingress Service.Update error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
				makeDifferentIngressService(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if svc, ok := obj.(*corev1.Service); ok {
							if svc.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
								return controllertesting.Handled, errors.New("test error updating ingress Service")
							}
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating ingress Service",
		},
		{
			Name:   "Broker.Get for status update fails",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					// The first Get works.
					func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, innerClient.Get(ctx, key, obj)
						}
						return controllertesting.Unhandled, nil
					},
					// The second Get fails.
					func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error getting the Broker for status update")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error getting the Broker for status update",
			WantEvent: []corev1.Event{
				{
					Reason: brokerReconciled, Type: corev1.EventTypeNormal,
				},
				{
					Reason: brokerUpdateStatusFailed, Type: corev1.EventTypeWarning,
				},
			},
		},
		{
			Name:   "Broker.Status.Update error",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				makeChannel(),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: []controllertesting.MockStatusUpdate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*v1alpha1.Broker); ok {
							return controllertesting.Handled, errors.New("test error updating the Broker status")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantErrMsg: "test error updating the Broker status",
			WantEvent: []corev1.Event{
				{
					Reason: brokerReconciled, Type: corev1.EventTypeNormal,
				},
				{
					Reason: brokerUpdateStatusFailed, Type: corev1.EventTypeWarning,
				},
			},
		},
		{
			Name:   "Successful reconcile",
			Scheme: scheme.Scheme,
			InitialState: []runtime.Object{
				makeBroker(),
				// The Channel needs to be addressable for the reconcile to succeed.
				makeChannel(),
			},
			WantPresent: []runtime.Object{
				makeReadyBroker(),
				// TODO Uncomment makeChannel() when our test framework handles generateName.
				// makeChannel(),
				makeFilterDeployment(),
				makeFilterService(),
				makeIngressDeployment(),
				makeIngressService(),
			},
			WantEvent: []corev1.Event{
				{
					Reason: brokerReconciled, Type: corev1.EventTypeNormal,
				},
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

			filterImage:               filterImage,
			filterServiceAccountName:  filterSA,
			ingressImage:              ingressImage,
			ingressServiceAccountName: ingressSA,
		}
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, brokerName)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      brokerName,
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{
				Provisioner: channelProvisioner,
			},
		},
	}
}

func makeReadyBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status.InitializeConditions()
	b.Status.MarkChannelReady()
	b.Status.SetAddress(fmt.Sprintf("%s-broker.%s.svc.%s", brokerName, testNS, utils.GetClusterDomainName()))
	b.Status.MarkFilterReady()
	b.Status.MarkIngressReady()
	return b
}

func makeDeletingBroker() *v1alpha1.Broker {
	b := makeReadyBroker()
	b.DeletionTimestamp = &deletionTime
	return b
}

func makeChannel() *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    testNS,
			GenerateName: fmt.Sprintf("%s-broker-", brokerName),
			Labels: map[string]string{
				"eventing.knative.dev/broker":           brokerName,
				"eventing.knative.dev/brokerEverything": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReference(),
			},
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: channelProvisioner,
		},
		Status: v1alpha1.ChannelStatus{
			Address: duckv1alpha1.Addressable{
				Hostname: channelHostname,
			},
		},
	}
}

func makeNonAddressableChannel() *v1alpha1.Channel {
	c := makeChannel()
	c.Status.Address = duckv1alpha1.Addressable{}
	return c
}

func makeDifferentChannel() *v1alpha1.Channel {
	c := makeChannel()
	c.Spec.Provisioner.Name = "some-other-provisioner"
	return c
}

func makeFilterDeployment() *appsv1.Deployment {
	d := resources.MakeFilterDeployment(&resources.FilterArgs{
		Broker:             makeBroker(),
		Image:              filterImage,
		ServiceAccountName: filterSA,
	})
	d.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
	return d
}

func makeDifferentFilterDeployment() *appsv1.Deployment {
	d := makeFilterDeployment()
	d.Spec.Template.Spec.Containers[0].Image = "some-other-image"
	return d
}

func makeFilterService() *corev1.Service {
	svc := resources.MakeFilterService(makeBroker())
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}
	return svc
}

func makeDifferentFilterService() *corev1.Service {
	s := makeFilterService()
	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
	return s
}

func makeIngressDeployment() *appsv1.Deployment {
	d := resources.MakeIngress(&resources.IngressArgs{
		Broker:             makeBroker(),
		Image:              ingressImage,
		ServiceAccountName: ingressSA,
		ChannelAddress:     channelHostname,
	})
	d.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
	return d
}

func makeDifferentIngressDeployment() *appsv1.Deployment {
	d := makeIngressDeployment()
	d.Spec.Template.Spec.Containers[0].Image = "some-other-image"
	return d
}

func makeIngressService() *corev1.Service {
	svc := resources.MakeIngressService(makeBroker())
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}
	return svc
}

func makeDifferentIngressService() *corev1.Service {
	s := makeIngressService()
	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
	return s
}

func getOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "Broker",
		Name:               brokerName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}
