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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
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
	ccpUid           = "test-uid"
	testErrorMessage = "test-induced-error"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		ccpReconciled:         {Reason: ccpReconciled, Type: corev1.EventTypeNormal},
		ccpReconcileFailed:    {Reason: ccpReconcileFailed, Type: corev1.EventTypeWarning},
		ccpUpdateStatusFailed: {Reason: ccpUpdateStatusFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
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
				Name:      Name,
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
				Name: Name,
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
						Name:      Name,
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
			Name: "Mark Ready",
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterChannelProvisioner(),
			},
			WantEvent: []corev1.Event{
				events[ccpReconciled],
			},
		},
		{
			Name: "Error getting CCP for updating Status",
			// Nothing to create or update other than the status of CCP itself.
			InitialState: []runtime.Object{
				makeClusterChannelProvisioner(),
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
			tc.ReconcileKey = fmt.Sprintf("/%s", Name)
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
			Name: Name,
			UID:  ccpUid,
		},
		Spec: eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
}

func makeReadyClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	ccp := makeClusterChannelProvisioner()
	ccp.Status.Conditions = []duckv1alpha1.Condition{{
		Type:     duckv1alpha1.ConditionReady,
		Status:   corev1.ConditionTrue,
		Severity: duckv1alpha1.ConditionSeverityError,
	}}
	return ccp
}

func makeDeletingClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	ccp := makeClusterChannelProvisioner()
	ccp.DeletionTimestamp = &deletionTime
	return ccp
}

func errorGettingClusterChannelProvisioner() controllertesting.MockGet {
	return func(client.Client, context.Context, client.ObjectKey, runtime.Object) (controllertesting.MockHandled, error) {
		return controllertesting.Handled, errors.New(testErrorMessage)
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
