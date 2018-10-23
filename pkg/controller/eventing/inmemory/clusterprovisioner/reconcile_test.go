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

package clusterprovisioner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/system"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	controllertesting "github.com/knative/eventing/pkg/controller/testing"
)

const (
	cpUid            = "test-uid"
	testErrorMessage = "test-induced-error"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	truePointer = true
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
		ref          *eventingv1alpha1.ProvisionerReference
		kind         string
		isControlled bool
	}{
		"nil": {
			ref:          nil,
			kind:         "Channel",
			isControlled: false,
		},
		"ref nil": {
			ref: &eventingv1alpha1.ProvisionerReference{
				Ref: nil,
			},
			kind:         "Channel",
			isControlled: false,
		},
		"wrong namespace": {
			ref: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Namespace: "other",
					Name:      Name,
				},
			},
			kind:         "Channel",
			isControlled: false,
		},
		"wrong name": {
			ref: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name: "other-name",
				},
			},
			kind:         "Channel",
			isControlled: false,
		},
		"wrong kind": {
			ref: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name: Name,
				},
			},
			kind:         "Source",
			isControlled: false,
		},
		"is controlled": {
			ref: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name: Name,
				},
			},
			kind:         "Channel",
			isControlled: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			isControlled := IsControlled(tc.ref, tc.kind)
			if isControlled != tc.isControlled {
				t.Errorf("Expected: %v. Actual: %v", tc.isControlled, isControlled)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "CP not found",
		},
		{
			Name: "Unable to get CP",
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					errorGettingClusterProvisioner(),
				},
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Should not reconcile - namespace",
			InitialState: []runtime.Object{
				&eventingv1alpha1.ClusterProvisioner{
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
				&eventingv1alpha1.ClusterProvisioner{
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
				makeDeletingClusterProvisioner(),
			},
		},
		{
			Name: "Create dispatcher fails",
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					errorGettingK8sService(),
				},
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Create dispatcher - already exists",
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
				makeK8sService(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterProvisioner(),
			},
		},
		{
			Name: "Create dispatcher - not owned by CP",
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
				makeK8sServiceNotOwnedByClusterProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterProvisioner(),
			},
		},
		{
			Name: "Create dispatcher succeeds",
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
			},
			WantPresent: []runtime.Object{
				makeReadyClusterProvisioner(),
				makeK8sService(),
			},
		},
		{
			Name: "Error getting CP for updating Status",
			// Nothing to create or update other than the status of CP itself.
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockGets: oneSuccessfulClusterProvisionerGet(),
			},
			WantErrMsg: testErrorMessage,
		},
		{
			Name: "Error updating Status",
			// Nothing to create or update other than the status of CP itself.
			InitialState: []runtime.Object{
				makeClusterProvisioner(),
				makeK8sService(),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					errorUpdating(),
				},
			},
			WantErrMsg: testErrorMessage,
		},
	}
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	for _, tc := range testCases {
		c := tc.GetClient()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),
		}
		if tc.ReconcileKey == "" {
			tc.ReconcileKey = fmt.Sprintf("/%s", Name)
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func makeClusterProvisioner() *eventingv1alpha1.ClusterProvisioner {
	return &eventingv1alpha1.ClusterProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterProvisioner",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: Name,
			UID:  cpUid,
		},
		Spec: eventingv1alpha1.ClusterProvisionerSpec{
			Reconciles: metav1.GroupKind{
				Group: "eventing.knative.dev/v1alpha1",
				Kind:  "Channel",
			},
		},
	}
}

func makeReadyClusterProvisioner() *eventingv1alpha1.ClusterProvisioner {
	cp := makeClusterProvisioner()
	cp.Status.Conditions = []duckv1alpha1.Condition{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
		},
	}
	return cp
}

func makeDeletingClusterProvisioner() *eventingv1alpha1.ClusterProvisioner {
	cp := makeClusterProvisioner()
	cp.DeletionTimestamp = &deletionTime
	return cp
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace,
			Name:      fmt.Sprintf("%s-clusterbus", Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterProvisioner",
					Name:               Name,
					UID:                cpUid,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
			Labels: dispatcherLabels(Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: dispatcherLabels(Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func makeK8sServiceNotOwnedByClusterProvisioner() *corev1.Service {
	svc := makeK8sService()
	svc.OwnerReferences = nil
	return svc
}

func errorGettingClusterProvisioner() controllertesting.MockGet {
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

func oneSuccessfulClusterProvisionerGet() []controllertesting.MockGet {
	return []controllertesting.MockGet{
		// The first one is a pass through.
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			err := innerClient.Get(ctx, key, obj)
			return controllertesting.Handled, err
		},
		// All subsequent ClusterProvisioner Gets fail.
		func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.ClusterProvisioner); ok {
				return controllertesting.Handled, errors.New(testErrorMessage)
			}
			return controllertesting.Unhandled, nil
		},
	}
}

func errorUpdating() controllertesting.MockUpdate {
	return func(client.Client, context.Context, runtime.Object) (controllertesting.MockHandled, error) {
		return controllertesting.Handled, errors.New(testErrorMessage)
	}
}
