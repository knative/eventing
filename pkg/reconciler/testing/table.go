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

package testing

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestCase holds a single row of our table test.
type TestCase struct {
	// Name is a descriptive name for this test suitable as a first argument to t.Run()
	Name string

	// InitialState is the list of objects that already exists when reconciliation
	// starts.
	InitialState []runtime.Object

	// ReconcileKey is the key of the object to reconcile in namespace/name form.
	ReconcileKey string

	// WantErr is true when we expect the Reconcile function to return an error.
	WantErr bool

	// WantErrMsg contains the pattern to match the returned error message.
	// Implies WantErr = true.
	WantErrMsg string

	// WantResult is the reconcile result we expect to be returned from the
	// Reconcile function.
	WantResult reconcile.Result

	// WantPresent holds the non-exclusive set of objects we expect to exist
	// after reconciliation completes.
	WantPresent []runtime.Object

	// WantAbsent holds the list of objects expected to not exist
	// after reconciliation completes.
	WantAbsent []runtime.Object

	// WantEvent holds the list of events expected to exist after
	// reconciliation completes.
	WantEvent []corev1.Event

	// Mocks that tamper with the client's responses.
	Mocks Mocks

	// Scheme for the dynamic client
	Scheme *runtime.Scheme

	// Fake dynamic objects
	Objects []runtime.Object

	// OtherTestData is arbitrary data needed for the test. It is not used directly by the table
	// testing framework. Instead it is used in the test method. E.g. setting up the responses for a
	// fake GCP PubSub client can go in here, as no other field makes sense for it.
	OtherTestData map[string]interface{}

	// AdditionalVerification is for any verification that needs to be done on top of the normal
	// result/error verification and WantPresent/WantAbsent.
	AdditionalVerification []func(t *testing.T, tc *TestCase)

	// IgnoreTimes causes comparisons to ignore fields of type apis.VolatileTime.
	IgnoreTimes bool
}

// Runner returns a testing func that can be passed to t.Run.
func (tc *TestCase) Runner(t *testing.T, r reconcile.Reconciler, c *MockClient, recorder *MockEventRecorder) func(t *testing.T) {
	return func(t *testing.T) {
		result, recErr := tc.Reconcile(r)

		if err := tc.VerifyErr(recErr); err != nil {
			t.Error(err)
		}

		if err := tc.VerifyResult(result); err != nil {
			t.Error(err)
		}

		// Verifying should be done against the innerClient, never against mocks.
		c.stopMocking()

		if err := tc.VerifyWantPresent(c); err != nil {
			t.Error(err)
		}

		if err := tc.VerifyWantAbsent(c); err != nil {
			t.Error(err)
		}

		if err := tc.VerifyWantEvent(recorder); err != nil {
			t.Error(err)
		}

		for _, av := range tc.AdditionalVerification {
			av(t, tc)
		}
	}
}

// GetDynamicClient returns the mockDynamicClient to use for this test case.
func (tc *TestCase) GetDynamicClient() dynamic.Interface {
	if tc.Scheme == nil {
		return dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), tc.Objects...)
	}
	return dynamicfake.NewSimpleDynamicClient(tc.Scheme, tc.Objects...)
}

// GetClient returns the mockClient to use for this test case.
func (tc *TestCase) GetClient() *MockClient {
	builtObjects := buildAllObjects(tc.InitialState)
	innerClient := fake.NewFakeClient(builtObjects...)
	return NewMockClient(innerClient, tc.Mocks)
}

// GetEventRecorder returns the mockEventRecorder to use for this test case.
func (tc *TestCase) GetEventRecorder() *MockEventRecorder {
	return NewEventRecorder()
}

// Reconcile calls the given reconciler's Reconcile() function with the test
// case's reconcile request.
func (tc *TestCase) Reconcile(r reconcile.Reconciler) (reconcile.Result, error) {
	if tc.ReconcileKey == "" {
		return reconcile.Result{}, fmt.Errorf("test did not set ReconcileKey")
	}
	ns, n, err := cache.SplitMetaNamespaceKey(tc.ReconcileKey)
	if err != nil {
		return reconcile.Result{}, err
	}
	return r.Reconcile(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: ns,
			Name:      n,
		},
	})
}

// VerifyErr verifies that the given error returned from Reconcile is the error
// expected by the test case.
func (tc *TestCase) VerifyErr(err error) error {
	// A non-empty WantErrMsg implies that an error is wanted.
	wantErr := tc.WantErr || tc.WantErrMsg != ""

	if wantErr && err == nil {
		return fmt.Errorf("want error, got nil")
	}

	if !wantErr && err != nil {
		return fmt.Errorf("want no error, got %v", err)
	}

	if err != nil {
		if diff := cmp.Diff(tc.WantErrMsg, err.Error()); diff != "" {
			return fmt.Errorf("incorrect error (-want, +got): %v", diff)
		}
	}
	return nil
}

// VerifyResult verifies that the given result returned from Reconcile is the
// result expected by the test case.
func (tc *TestCase) VerifyResult(result reconcile.Result) error {
	if diff := cmp.Diff(tc.WantResult, result); diff != "" {
		return fmt.Errorf("unexpected reconcile Result (-want +got) %v", diff)
	}
	return nil
}

type stateErrors struct {
	errors []error
}

func (se stateErrors) Error() string {
	msgs := make([]string, 0)
	for _, err := range se.errors {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}

// VerifyWantPresent verifies that the client contains all the objects expected
// to be present after reconciliation.
func (tc *TestCase) VerifyWantPresent(c client.Client) error {
	var errs stateErrors
	builtObjects := buildAllObjects(tc.WantPresent)
	for _, wp := range builtObjects {
		o, err := scheme.Scheme.New(wp.GetObjectKind().GroupVersionKind())
		if err != nil {
			errs.errors = append(errs.errors, fmt.Errorf("error creating a copy of %T: %v", wp, err))
		}
		acc, err := meta.Accessor(wp)
		if err != nil {
			errs.errors = append(errs.errors, fmt.Errorf("error getting accessor for %#v %v", wp, err))
		}
		err = c.Get(context.TODO(), client.ObjectKey{Namespace: acc.GetNamespace(), Name: acc.GetName()}, o)
		if err != nil {
			if apierrors.IsNotFound(err) {
				errs.errors = append(errs.errors, fmt.Errorf("want present %T %s/%s, got absent", wp, acc.GetNamespace(), acc.GetName()))
			} else {
				errs.errors = append(errs.errors, fmt.Errorf("error getting %T %s/%s: %v", wp, acc.GetNamespace(), acc.GetName(), err))
			}
		}

		diffOpts := cmp.Options{
			// Ignore TypeMeta, since the objects created by the controller won't have
			// it
			cmpopts.IgnoreTypes(metav1.TypeMeta{}),
		}

		if tc.IgnoreTimes {
			// Ignore VolatileTime fields, since they rarely compare correctly.
			diffOpts = append(diffOpts, cmpopts.IgnoreTypes(apis.VolatileTime{}))
		}

		if diff := cmp.Diff(wp, o, diffOpts...); diff != "" {
			errs.errors = append(errs.errors, fmt.Errorf("Unexpected present %T %s/%s (-want +got):\n%v", wp, acc.GetNamespace(), acc.GetName(), diff))
		}
	}
	if len(errs.errors) > 0 {
		return errs
	}
	return nil
}

// VerifyWantAbsent verifies that the client does not contain any of the objects
// expected to be absent after reconciliation.
func (tc *TestCase) VerifyWantAbsent(c client.Client) error {
	var errs stateErrors
	for _, wa := range tc.WantAbsent {
		acc, err := meta.Accessor(wa)
		if err != nil {
			errs.errors = append(errs.errors, fmt.Errorf("error getting accessor for %#v %v", wa, err))
		}
		err = c.Get(context.TODO(), client.ObjectKey{Namespace: acc.GetNamespace(), Name: acc.GetName()}, wa)
		if err == nil {
			errs.errors = append(errs.errors, fmt.Errorf("want absent, got present %T %s/%s", wa, acc.GetNamespace(), acc.GetName()))
		}
		if !apierrors.IsNotFound(err) {
			errs.errors = append(errs.errors, fmt.Errorf("error getting %T %s/%s: %v", wa, acc.GetNamespace(), acc.GetName(), err))
		}
	}
	if len(errs.errors) > 0 {
		return errs
	}
	return nil
}

// VerifyWantEvent verifies that the eventRecorder does contain the events
// expected in the same order as they were emitted after reconciliation.
func (tc *TestCase) VerifyWantEvent(eventRecorder *MockEventRecorder) error {
	if !reflect.DeepEqual(tc.WantEvent, eventRecorder.events) {
		return fmt.Errorf("expected %s, got %s", getEventsAsString(tc.WantEvent), getEventsAsString(eventRecorder.events))
	}
	return nil
}

func getEventsAsString(events []corev1.Event) []string {
	eventsAsString := make([]string, 0, len(events))
	for _, event := range events {
		eventsAsString = append(eventsAsString, fmt.Sprintf("(%s,%s)", event.Reason, event.Type))
	}
	return eventsAsString
}

func buildAllObjects(objs []runtime.Object) []runtime.Object {
	builtObjs := []runtime.Object{}
	for _, obj := range objs {
		if builder, ok := obj.(Buildable); ok {
			obj = builder.Build()
		}
		builtObjs = append(builtObjs, obj)
	}
	return builtObjs
}
