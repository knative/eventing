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

package channel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testNamespace  = "testnamespace"
	testAPIVersion = "eventing.knative.dev/v1alpha1"
	testCCPName    = "TestProvisioner"
	testCCPKind    = "ClusterChannelProvisioner"
)

var (
	events = map[string]corev1.Event{
		channelReconciled:         {Reason: channelReconciled, Type: corev1.EventTypeNormal},
		channelUpdateStatusFailed: {Reason: channelUpdateStatusFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
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

func TestAllCases(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:         "No channels exist",
			WantResult:   reconcile.Result{},
			ReconcileKey: fmt.Sprintf("%v/%v", "chan-1", testNamespace),
		}, {
			Name:         "Cannot get Channel",
			WantResult:   reconcile.Result{},
			ReconcileKey: fmt.Sprintf("%v/%v", "chan-1", testNamespace),
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{accessDenied},
			},
			WantErrMsg: "access denied",
		}, {
			Name:         "Orphaned channel",
			WantResult:   reconcile.Result{},
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, "chan-1"),
			InitialState: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantPresent: []runtime.Object{
				Channel("chan-1", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		}, {
			Name:         "Non-orphaned channel test 1",
			WantResult:   reconcile.Result{},
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, "chan-2"),
			InitialState: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantPresent: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		}, {
			Name:         "Non-orphaned channel test 2",
			WantResult:   reconcile.Result{},
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, "chan-3"),
			InitialState: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantPresent: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantEvent: []corev1.Event{
				events[channelReconciled],
			},
		}, {
			Name:         "Fail orphaned channel status update",
			WantErrMsg:   "update failed",
			WantResult:   reconcile.Result{Requeue: true},
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, "chan-1"),
			InitialState: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantPresent: []runtime.Object{
				Channel("chan-1", testNamespace),
				Channel("chan-2", testNamespace).WithProvInstalledStatus(corev1.ConditionTrue),
				Channel("chan-3", testNamespace).WithProvInstalledStatus(corev1.ConditionFalse),
			},
			WantEvent: []corev1.Event{
				events[channelUpdateStatusFailed],
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: []controllertesting.MockStatusUpdate{failUpdate},
			},
		},
	}
	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()
		recorder := tc.GetEventRecorder()

		r := &reconciler{
			client:        c,
			dynamicClient: dc,
			restConfig:    &rest.Config{},
			recorder:      recorder,
			logger:        zap.NewNop(),
		}
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func failUpdate(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
	return controllertesting.Handled, errors.New("update failed")
}

func accessDenied(_ client.Client, _ context.Context, _ client.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
	return controllertesting.Handled, errors.New("access denied")
}

type ChannelBuilder struct {
	*eventingv1alpha1.Channel
}

// Verify the Builder implements Buildable
var _ controllertesting.Buildable = &ChannelBuilder{}

func (s *ChannelBuilder) Build() runtime.Object {
	return s.Channel
}

func Channel(name string, namespace string) *ChannelBuilder {
	channel := getTestChannelWithoutStatus(name, namespace)
	return &ChannelBuilder{
		Channel: channel,
	}
}

func (cb *ChannelBuilder) WithProvInstalledStatus(provInstalledStatus corev1.ConditionStatus) *ChannelBuilder {
	cb.Status = eventingv1alpha1.ChannelStatus{}

	switch provInstalledStatus {
	case corev1.ConditionTrue:
		cb.Status.MarkProvisionerInstalled()
		break
	case corev1.ConditionFalse:
		cb.Status.MarkProvisionerNotInstalled(
			"Provisioner not found.",
			"Specified provisioner [Name:%v Kind:%v] is not installed or not controlling the channel.",
			testCCPName,
			testCCPKind,
		)
		break
	}
	return cb
}

func getTestChannelWithoutStatus(name string, namespace string) *eventingv1alpha1.Channel {
	ch := &eventingv1alpha1.Channel{}
	ch.APIVersion = testAPIVersion
	ch.Namespace = testNamespace
	ch.Kind = "Channel"
	ch.Name = name
	ch.Namespace = namespace
	ch.Spec = getTestChannelSpec()
	return ch
}

func getTestChannelSpec() eventingv1alpha1.ChannelSpec {
	chSpec := eventingv1alpha1.ChannelSpec{
		Provisioner: &corev1.ObjectReference{},
		Subscribable: &eventingduck.Subscribable{
			Subscribers: []eventingduck.ChannelSubscriberSpec{
				getTestSubscriberSpec(),
				getTestSubscriberSpec(),
			},
		},
	}
	chSpec.Provisioner.Name = testCCPName
	chSpec.Provisioner.APIVersion = testAPIVersion
	chSpec.Provisioner.Kind = testCCPKind
	return chSpec
}

func getTestSubscriberSpec() eventingduck.ChannelSubscriberSpec {
	return eventingduck.ChannelSubscriberSpec{
		SubscriberURI: "TestSubscriberURI",
		ReplyURI:      "TestReplyURI",
	}
}
