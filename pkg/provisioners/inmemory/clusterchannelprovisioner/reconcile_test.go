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

package clusterchannelprovisioner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/knative/pkg/apis"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/system"
	_ "github.com/knative/pkg/system/testing"
)

const (
	ccpUID             = "test-uid"
	testErrorMessage   = "test-induced-error"
	testNS             = "test-ns"
	dispatcherPodLabel = "in-memory-channel"
	inMemoryName       = "in-memory"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		ccpReconciled:          {Reason: ccpReconciled, Type: corev1.EventTypeNormal},
		ccpUpdateStatusFailed:  {Reason: ccpUpdateStatusFailed, Type: corev1.EventTypeWarning},
		k8sServiceCreateFailed: {Reason: k8sServiceCreateFailed, Type: corev1.EventTypeWarning},
		k8sServiceDeleteFailed: {Reason: k8sServiceDeleteFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
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

func TestIsControlled(t *testing.T) {
	testCases := map[string]struct {
		ref          *corev1.ObjectReference
		isControlled bool
	}{
		"nil": {
			ref:          nil,
			isControlled: false,
		},
		"wrong namespace": {
			ref: &corev1.ObjectReference{
				Namespace: "other",
				Name:      inMemoryName,
			},
			isControlled: false,
		},
		"wrong name": {
			ref: &corev1.ObjectReference{
				Name: "other-name",
			},
			isControlled: false,
		},
		"is controlled": {
			ref: &corev1.ObjectReference{
				Name: inMemoryName,
			},
			isControlled: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			isControlled := IsControlled(tc.ref)
			if isControlled != tc.isControlled {
				t.Errorf("Expected: %v. Actual: %v", tc.isControlled, isControlled)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "CCP not found",
		},
		{
			Name: "Unable to get CCP",
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					errorGettingClusterChannelProvisioner(),
				},
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Should not reconcile - namespace",
			InitialState: []runtime.Object{
				&eventingv1alpha1.ClusterChannelProvisioner{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "not empty string",
						Name:      inMemoryName,
					},
				},
			},
		},
		{
			Name: "Should not reconcile - name",
			InitialState: []runtime.Object{
				&eventingv1alpha1.ClusterChannelProvisioner{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wrong-name",
					},
				},
			},
			ReconcileKey: "/wrong-name",
		},
		{
			Name: "Delete succeeds",
			// Deleting does nothing.
			InitialState: []runtime.Object{
				makeDeletingClusterChannelProvisioner(),
			},
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Create dispatcher fails",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					errorGettingK8sService(),
				},
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[k8sServiceCreateFailed],
			},
		},
		{
			Name: "Create dispatcher - already exists",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
				makeK8sService(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterChannelProvisioner(),
			},
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Create dispatcher - not owned by CCP",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
				makeK8sServiceNotOwnedByClusterChannelProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterChannelProvisioner(),
			},
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Create dispatcher succeeds",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterChannelProvisioner(),
				makeK8sService(),
			},
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Create dispatcher succeeds - request is namespace-scoped",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterChannelProvisioner(),
				makeK8sService(),
			},
			ReconcileKey: fmt.Sprintf("%s/%s", testNS, inMemoryName),
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Error getting CCP for updating Status",
			// Nothing to create or update other than the status of CCP itself.
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: oneSuccessfulClusterChannelProvisionerGet(),
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[ccpReconciled], events[ccpUpdateStatusFailed],
			},
		},
		{
			Name: "Error updating Status",
			// Nothing to create or update other than the status of CCP itself.
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: []controllertesting.MockStatusUpdate{
					errorUpdatingStatus(),
				},
			},
			WantErrMsg: testErrorMessage,
			WantEvent: []corev1.Event{
				events[ccpReconciled], events[ccpUpdateStatusFailed],
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
			tc.ReconcileKey = fmt.Sprintf("/%s", inMemoryName)
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func makeClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	return &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterChannelProvisioner",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: inMemoryName,
			UID:  ccpUID,
		},
		Spec: eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
}

func makeReadyClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	ccp := makeClusterChannelProvisioner()
	ccp.Status.Conditions = []apis.Condition{{
		Type:     apis.ConditionReady,
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityError,
	}}
	return ccp
}

func makeDeletingClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	ccp := makeClusterChannelProvisioner()
	ccp.DeletionTimestamp = &deletionTime
	return ccp
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      fmt.Sprintf("%s-dispatcher", inMemoryName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterChannelProvisioner",
					Name:               inMemoryName,
					UID:                ccpUID,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
			Labels: util.DispatcherLabels(inMemoryName),
		},
		Spec: corev1.ServiceSpec{
			Selector: util.DispatcherLabels(dispatcherPodLabel),
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func makeK8sServiceNotOwnedByClusterChannelProvisioner() *corev1.Service {
	svc := makeK8sService()
	svc.OwnerReferences = nil
	return svc
}

func errorGettingClusterChannelProvisioner() controllertesting.MockGet {
	return func(client.Client, context.Context, client.ObjectKey, runtime.Object) (controllertesting.MockHandled, error) {
		return controllertesting.Handled, errors.New(testErrorMessage)
	}
}

func errorGettingK8sService() controllertesting.MockGet {
	return func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		if _, ok := obj.(*corev1.Service); ok {
			return controllertesting.Handled, errors.New(testErrorMessage)
		}
		return controllertesting.Unhandled, nil
	}
}

func oneSuccessfulClusterChannelProvisionerGet() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		// The first one is a pass through.
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			err := innerClient.Get(ctx, key, obj)
			return controllertesting.Handled, err
		},
		// All subsequent ClusterChannelProvisioner Gets fail.
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.ClusterChannelProvisioner); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

func errorUpdatingStatus() controllertesting.MockStatusUpdate {
	return func(client.Client, context.Context, runtime.Object) (controllertesting.MockHandled, error) {
		return controllertesting.Handled, errors.New(testErrorMessage)
	}
}
