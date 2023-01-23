/*
Copyright 2020 The Knative Authors

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

package feature

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/injection/clients/dynamicclient"

	"knative.dev/reconciler-test/pkg/state"
)

// Feature is a list of steps and feature name.
type Feature struct {
	Name  string
	Steps []Step
	State state.Store
	// Contains all the resources created as part of this Feature.
	refs   []corev1.ObjectReference
	refsMu sync.Mutex
}

func (f *Feature) MarshalJSON() ([]byte, error) {
	return f.marshalJSON(true)
}

func (f *Feature) marshalJSON(pretty bool) ([]byte, error) {
	f.refsMu.Lock()
	defer f.refsMu.Unlock()

	in := struct {
		Name  string                   `json:"name"`
		Steps []Step                   `json:"steps"`
		State state.Store              `json:"state"`
		Refs  []corev1.ObjectReference `json:"refs"`
	}{
		Name:  f.Name,
		Steps: f.Steps,
		State: f.State,
		Refs:  f.refs,
	}
	marshaler := json.MarshalIndent
	if !pretty {
		marshaler = func(in interface{}, _, _ string) ([]byte, error) {
			return json.Marshal(in)
		}
	}
	return marshaler(in, "", " ")
}

// DumpWith calls the provided log function with a nicely formatted string
// that represents the Feature.
func (f *Feature) DumpWith(log func(args ...interface{})) {
	b, err := f.marshalJSON(false)
	if err != nil {
		log("Skipping feature logging due to error: ", err.Error())
	}
	log("Feature state: ", string(b))
}

// NewFeatureNamed creates a new feature with the provided name
func NewFeatureNamed(name string) *Feature {
	f := new(Feature)
	f.Name = name
	return f
}

// NewFeature creates a new feature with name taken from the caller name
func NewFeature() *Feature {
	f := new(Feature)

	pc, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc)
	if caller != nil {
		splitted := strings.Split(caller.Name(), ".")
		f.Name = splitted[len(splitted)-1]
	}

	return f
}

// FeatureSet is a list of features and feature set name.
type FeatureSet struct {
	Name     string
	Features []*Feature
}

// StepFn is the function signature for steps.
type StepFn func(ctx context.Context, t T)

// Step is a structure to hold the step function, step name and state, level and
// timing configuration.
type Step struct {
	Name string `json:"name"`
	S    States `json:"states"`
	L    Levels `json:"levels"`
	T    Timing `json:"timing"`
	Fn   StepFn `json:"-"`
}

type Steps []Step

func (ss Steps) String() string {
	bytes, _ := json.MarshalIndent(ss, "", "  ")
	return string(bytes)
}

// TestName returns the constructed test name based on the timing, step, state,
// level, and the Name provided in the step.
func (s *Step) TestName() string {
	switch s.T {
	case Assert:
		return fmt.Sprintf("[%s/%s]%s", s.S, s.L, s.Name)
	default:
		return s.Name
	}
}

// Reference adds references to keep track of for example, for cleaning things
// after a Feature completes.
func (f *Feature) Reference(ref ...corev1.ObjectReference) {
	f.refsMu.Lock()
	defer f.refsMu.Unlock()

	f.refs = append(f.refs, ref...)
}

// References returns all known resources to the Feature registered via
// `Reference`.
func (f *Feature) References() []corev1.ObjectReference {
	f.refsMu.Lock()
	defer f.refsMu.Unlock()

	r := make([]corev1.ObjectReference, len(f.refs))
	copy(r, f.refs)
	return r
}

// DeleteResources delete all known resources to the Feature registered
// via `Reference`.
//
// It doesn't fail when a referenced resource couldn't be deleted.
// Use References to get the undeleted resources.
//
// Expected to be used as a StepFn.
func (f *Feature) DeleteResources(ctx context.Context, t T) {
	dc := dynamicclient.Get(ctx)
	for _, ref := range f.References() {

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			t.Fatalf("Could not parse GroupVersion for %+v", ref.APIVersion)
		}

		resource := apis.KindToResource(gv.WithKind(ref.Kind))
		t.Logf("Deleting %s/%s of GVR: %+v", ref.Namespace, ref.Name, resource)

		deleteOptions := &metav1.DeleteOptions{}
		// Set delete propagation policy to foreground
		foregroundDeletePropagation := metav1.DeletePropagationForeground
		deleteOptions.PropagationPolicy = &foregroundDeletePropagation

		err = dc.Resource(resource).Namespace(ref.Namespace).Delete(ctx, ref.Name, *deleteOptions)
		// Ignore not found errors.
		if err != nil && !apierrors.IsNotFound(err) {
			t.Logf("Warning, failed to delete %s/%s of GVR: %+v: %v", ref.Namespace, ref.Name, resource, err)
		}
	}

	// refFailedDeletion keeps the failed to delete resources.
	var refFailedDeletion []corev1.ObjectReference

	err := wait.Poll(time.Second, 4*time.Minute, func() (bool, error) {
		refFailedDeletion = nil // Reset failed deletion.
		for _, ref := range f.References() {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				t.Fatalf("Could not parse GroupVersion for %+v", ref.APIVersion)
			}

			resource := apis.KindToResource(gv.WithKind(ref.Kind))
			t.Logf("Deleting %s/%s of GVR: %+v", ref.Namespace, ref.Name, resource)

			_, err = dc.Resource(resource).
				Namespace(ref.Namespace).
				Get(ctx, ref.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				refFailedDeletion = append(refFailedDeletion, ref)
				return false, fmt.Errorf("failed to get resource %+v %s/%s: %w", resource, ref.Namespace, ref.Name, err)
			}

			t.Logf("Resource %+v %s/%s still present", resource, ref.Namespace, ref.Name)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		LogReferences(refFailedDeletion...)(ctx, t)
		t.Fatalf("failed to wait for resources to be deleted: %v", err)
	}

	f.refsMu.Lock()
	defer f.refsMu.Unlock()

	f.refs = refFailedDeletion
}

var (
	// Expected to be used as a StepFn.
	_ StepFn = (&Feature{}).DeleteResources
)

// Setup adds a step function to the feature set at the Setup timing phase.
func (f *Feature) Setup(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Setup,
		Fn:   fn,
	})
}

// Requirement adds a step function to the feature set at the Requirement timing phase.
func (f *Feature) Requirement(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Requirement,
		Fn:   fn,
	})
}

// Assert is a shortcut for Stable().Must(name, fn),
// useful for developing integration tests that don't require assertion levels.
func (f *Feature) Assert(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Stable,
		L:    Must,
		T:    Assert,
		Fn:   fn,
	})
}

// Assert adds a step function to the feature set at the Assert timing phase.
func (a *Asserter) Assert(l Levels, name string, fn StepFn) {
	a.f.AddStep(Step{
		Name: fmt.Sprintf("%s %s", a.name, name),
		S:    a.s,
		L:    l,
		T:    Assert,
		Fn:   fn,
	})
}

// Teardown adds a step function to the feature set at the Teardown timing phase.
func (f *Feature) Teardown(name string, fn StepFn) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Teardown,
		Fn:   fn,
	})
}

// AddStep appends one or more steps to the Feature.
func (f *Feature) AddStep(step ...Step) {
	f.Steps = append(f.Steps, step...)
}

// Assertable is a fluent interface based on Levels for creating an Assert step.
type Assertable interface {
	Must(name string, fn StepFn) Assertable
	Should(name string, fn StepFn) Assertable
	May(name string, fn StepFn) Assertable
	MustNot(name string, fn StepFn) Assertable
	ShouldNot(name string, fn StepFn) Assertable
}

// Alpha is a fluent style method for creating an Assert step in Alpha State.
func (f *Feature) Alpha(name string) Assertable {
	return f.asserter(Alpha, name)
}

// Beta is a fluent style method for creating an Assert step in Beta State.
func (f *Feature) Beta(name string) Assertable {
	return f.asserter(Beta, name)
}

// Stable is a fluent style method for creating an Assert step in Stable State.
func (f *Feature) Stable(name string) Assertable {
	return f.asserter(Stable, name)
}

func (f *Feature) asserter(s States, name string) Assertable {
	return &Asserter{
		f:    f,
		name: name,
		s:    s,
	}
}

type Asserter struct {
	f    *Feature
	name string
	s    States
}

func (a *Asserter) Must(name string, fn StepFn) Assertable {
	a.Assert(Must, name, fn)
	return a
}

func (a *Asserter) Should(name string, fn StepFn) Assertable {
	a.Assert(Should, name, fn)
	return a
}

func (a *Asserter) May(name string, fn StepFn) Assertable {
	a.Assert(May, name, fn)
	return a
}

func (a *Asserter) MustNot(name string, fn StepFn) Assertable {
	a.Assert(MustNot, name, fn)
	return a
}

func (a *Asserter) ShouldNot(name string, fn StepFn) Assertable {
	a.Assert(ShouldNot, name, fn)
	return a
}
