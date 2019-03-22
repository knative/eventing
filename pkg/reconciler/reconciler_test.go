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

package reconciler

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	testNamespace     = "testnamespace"
	testChannel       = "testchannel"
	testAPIVersion    = "eventing.knative.dev/v1alpha1"
	testCCPName       = "TestProvisioner"
	testCCPKind       = "ClusterChannelProvisioner"
	reconcilerKey     = "reconciler"
	testFinalizerName = "test-finalizer"
)

var (
	events = map[string]corev1.Event{
		Reconciled:            {Reason: Reconciled, Type: corev1.EventTypeNormal},
		AddFinalizerFailed:    {Reason: AddFinalizerFailed, Type: corev1.EventTypeWarning},
		RemoveFinalizerFailed: {Reason: RemoveFinalizerFailed, Type: corev1.EventTypeWarning},
		UpdateStatusFailed:    {Reason: UpdateStatusFailed, Type: corev1.EventTypeWarning},
		ReconcileFailed:       {Reason: ReconcileFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

type NewFuncTestCase struct {
	name           string
	nilLogger      bool
	nilRecorder    bool
	r              EventingReconciler
	opts           []option
	expectedErrMsg string
}

func TestNew(t *testing.T) {
	testCases := []NewFuncTestCase{
		{
			name:           "Nil recorder",
			nilRecorder:    true,
			r:              &MockReconciler{},
			opts:           []option{EnableConfigInjection(), EnableFilter(), EnableFinalizer(testFinalizerName)},
			expectedErrMsg: "recorder is nil",
		},
		{
			name:           "nil reconciler",
			r:              nil,
			opts:           []option{EnableConfigInjection(), EnableFilter(), EnableFinalizer(testFinalizerName)},
			expectedErrMsg: "EventingReconciler is nil",
		},
		{
			name:           "nil logger",
			nilLogger:      true,
			r:              &MockReconciler{},
			opts:           []option{EnableConfigInjection(), EnableFilter(), EnableFinalizer(testFinalizerName)},
			expectedErrMsg: "logger is nil",
		},
		{
			name: "Success",
			r:    &MockReconciler{},
			opts: []option{EnableConfigInjection(), EnableFilter(), EnableFinalizer(testFinalizerName)},
		},
		{
			name:           "Check inject.Config",
			r:              &MockReconcilerInterfaceOnly{EventingReconciler: &MockReconciler{}},
			opts:           []option{EnableConfigInjection()},
			expectedErrMsg: "EventingReconciler doesn't implement inject.Config interface",
		},
		{
			name:           "Check Filter",
			r:              &MockReconcilerInterfaceOnly{EventingReconciler: &MockReconciler{}},
			opts:           []option{EnableFilter()},
			expectedErrMsg: "EventingReconciler doesn't implement Filter interface",
		},
		{
			name:           "Check finalizer",
			r:              &MockReconcilerInterfaceOnly{EventingReconciler: &MockReconciler{}},
			opts:           []option{EnableFinalizer(testFinalizerName)},
			expectedErrMsg: "EventingReconciler doesn't implement Finalizer interface",
		},
	}
	for _, tc := range testCases {

		var logger *zap.Logger
		var recorder record.EventRecorder

		if !tc.nilLogger {
			logger = zap.NewNop()
		}
		if !tc.nilRecorder {
			recorder = &controllertesting.MockEventRecorder{}
		}

		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.r, logger, recorder, tc.opts...)
			if err != nil && tc.expectedErrMsg != err.Error() {
				t.Log(fmt.Sprintf("Expected: %v, Got: %v", tc.expectedErrMsg, err))
				t.Fail()
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:         "Modify request to non-existant channel",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus()),
			},
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					rmfunc: RequestModifierFunc(modifyrequest),
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus()),
			},
		},
		{
			Name:         "client get fails",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus()),
			},
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{accessDenied},
			},
			WantErrMsg: "access denied",
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus()),
			},
		},
		{
			Name:         "Should reconcile returns false",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus()),
			},
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					srfunc: func(context.Context, ReconciledResource, record.EventRecorder) bool { return false },
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus()),
			},
		},
		{
			Name:         "Deleted Channel - update failed",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withFinalizer()),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{failUpdate},
			},
			WantResult: reconcile.Result{},
			WantErrMsg: "update failed",
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[RemoveFinalizerFailed],
			},
		},
		{
			Name:         "Deleted Channel - finalizer/OnDelete failed",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withFinalizer()),
			},
			WantResult: reconcile.Result{},
			WantErrMsg: "OnDelete() failed",
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
					odfunc:        onDeleteFail,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[RemoveFinalizerFailed],
			},
		},
		{
			Name:         "Deleted Channel - success",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withFinalizer()),
			},
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), deleted(), withoutFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[Reconciled],
			},
		},
		{
			Name:         "Add finalizer - update fails",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{failUpdate},
			},
			WantErrMsg: "update failed",
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[AddFinalizerFailed],
			},
		},
		{
			Name:         "Reconcile failed - inner reconcile failed while status and finalizer were updated",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			WantErrMsg: "reconcile failed",
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
					recfunc:       reconcileFailUpdateStatus,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withStatus(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[ReconcileFailed],
			},
		},
		{
			Name:         "Reconcile failed - do not update status",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			WantErrMsg: "reconcile failed",
			WantResult: reconcile.Result{Requeue: true},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
					recfunc:       reconcileFailRequeue,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[ReconcileFailed],
			},
		},
		{
			Name:         "Reconcile failed - status update failed",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			Mocks: controllertesting.Mocks{
				MockStatusUpdates: []controllertesting.MockStatusUpdate{failStatusUpdate},
			},
			WantErrMsg: "status update failed",
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withoutStatus(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[Reconciled],
				events[UpdateStatusFailed],
			},
		},
		{
			Name:         "Reconcile succeeded",
			ReconcileKey: fmt.Sprintf("%v/%v", testNamespace, testChannel),
			InitialState: []runtime.Object{
				NewChannel(withoutStatus(), withoutFinalizer()),
			},
			WantResult: reconcile.Result{},
			OtherTestData: map[string]interface{}{
				reconcilerKey: &MockReconciler{
					finalizerName: testFinalizerName,
				},
			},
			WantPresent: []runtime.Object{
				NewChannel(withStatus(), withFinalizer()),
			},
			WantEvent: []corev1.Event{
				events[Reconciled],
			},
		},
	}
	for _, tc := range testCases {
		c := tc.GetClient()
		recorder := tc.GetEventRecorder()

		mr, ok := tc.OtherTestData[reconcilerKey]
		if !ok {
			t.FailNow()
		}
		r := buildReconciler(mr.(*MockReconciler), zap.NewNop(), recorder)
		tc.IgnoreTimes = true
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func buildReconciler(er *MockReconciler, logger *zap.Logger, recorder record.EventRecorder) reconcile.Reconciler {
	r := &reconciler{
		EventingReconciler: er,
		logger:             logger,
		recorder:           recorder,
		finalizerName:      er.finalizerName,
	}
	if er.rmfunc != nil {
		opt := EnableFilter()
		opt(r)
	}
	if er.srfunc != nil {
		opt := EnableFilter()
		opt(r)
	}
	if er.odfunc != nil {
		opt := EnableFinalizer(er.finalizerName)
		opt(r)
	}
	if er.rmfunc != nil {
		opt := ModifyRequest(er.rmfunc)
		opt(r)
	}
	return r
}

// verify MockReconciler implements EventingReconciler only
var _ EventingReconciler = &MockReconciler{}

type MockReconcilerInterfaceOnly struct {
	EventingReconciler
}

// verify MockReconciler implements all necessary interfaces
var _ EventingReconciler = &MockReconciler{}
var _ Finalizer = &MockReconciler{}
var _ Filter = &MockReconciler{}
var _ inject.Config = &MockReconciler{}

type MockReconciler struct {
	client        client.Client
	config        *rest.Config
	recfunc       reconcileResourceFunc
	grcfunc       getReconcileResourceFunc
	iclientfunc   injectClientFunc
	odfunc        onDeleteFunc
	finalizerName string
	srfunc        shouldReconcileFunc
	iconfigfunc   injectConfigFunc
	rmfunc        RequestModifierFunc
}

func (r *MockReconciler) ReconcileResource(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	if r.recfunc != nil {
		return r.recfunc(ctx, obj, recorder)
	}
	c := obj.(*eventingv1alpha1.Channel)
	c.Status.InitializeConditions()
	return true, reconcile.Result{}, nil
}

func (r *MockReconciler) GetNewReconcileObject() ReconciledResource {
	if r.grcfunc != nil {
		return r.grcfunc()
	}
	return &eventingv1alpha1.Channel{}
}

func (r *MockReconciler) InjectClient(c client.Client) error {
	if r.iclientfunc != nil {
		return r.iclientfunc(c)
	}
	r.client = c
	return nil
}

func (r *MockReconciler) OnDelete(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) error {
	if r.odfunc != nil {
		return r.odfunc(ctx, obj, recorder)
	}
	return nil
}

func (r *MockReconciler) ShouldReconcile(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) bool {
	if r.srfunc != nil {
		return r.srfunc(ctx, obj, recorder)
	}
	return true
}

func (r *MockReconciler) InjectConfig(c *rest.Config) error {
	return nil
}

func reconcileFailRequeue(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	c := obj.(*eventingv1alpha1.Channel)
	c.Status.InitializeConditions()
	return false, reconcile.Result{Requeue: true}, errors.New("reconcile failed")
}

func reconcileFailUpdateStatus(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	c := obj.(*eventingv1alpha1.Channel)
	c.Status.InitializeConditions()
	return true, reconcile.Result{}, errors.New("reconcile failed")
}
func onDeleteFail(ctx context.Context, obj ReconciledResource, recorder record.EventRecorder) error {
	return errors.New("OnDelete() failed")
}

func modifyrequest(req *reconcile.Request) {
	req.Namespace = "NonExistingNamespace"
}

func (r *MockReconciler) injectConfig(c *rest.Config) error {
	if r.iclientfunc != nil {
		return r.iconfigfunc(c)
	}
	r.config = c
	return nil
}

type reconcileResourceFunc func(context.Context, ReconciledResource, record.EventRecorder) (bool, reconcile.Result, error)
type getReconcileResourceFunc func() ReconciledResource
type injectClientFunc func(client.Client) error
type onDeleteFunc func(context.Context, ReconciledResource, record.EventRecorder) error
type shouldReconcileFunc func(context.Context, ReconciledResource, record.EventRecorder) bool
type injectConfigFunc func(*rest.Config) error

type channeloption func(*eventingv1alpha1.Channel)

func NewChannel(opts ...channeloption) *eventingv1alpha1.Channel {
	c := getChannelLite()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func withStatus() channeloption {
	return func(c *eventingv1alpha1.Channel) {
		c.Status.InitializeConditions()
	}
}

func withFinalizer() channeloption {
	return func(c *eventingv1alpha1.Channel) {
		c.Finalizers = []string{testFinalizerName}
	}
}

func withoutFinalizer() channeloption {
	return func(c *eventingv1alpha1.Channel) {
		c.Finalizers = nil
	}
}

func deleted() channeloption {
	return func(c *eventingv1alpha1.Channel) {
		deletionTime := metav1.Now().Rfc3339Copy()
		c.DeletionTimestamp = &deletionTime
	}
}

func withoutStatus() channeloption {
	return func(ch *eventingv1alpha1.Channel) {
		ch.Status = eventingv1alpha1.ChannelStatus{}
	}
}

func getChannelLite() *eventingv1alpha1.Channel {
	ch := &eventingv1alpha1.Channel{}
	ch.Name = testChannel
	ch.Namespace = testNamespace
	ch.APIVersion = testAPIVersion
	ch.Namespace = testNamespace
	ch.Kind = "Channel"
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

func failStatusUpdate(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
	return controllertesting.Handled, errors.New("status update failed")
}

func failUpdate(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
	return controllertesting.Handled, errors.New("update failed")
}

func accessDenied(_ client.Client, _ context.Context, _ client.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
	return controllertesting.Handled, errors.New("access denied")
}
