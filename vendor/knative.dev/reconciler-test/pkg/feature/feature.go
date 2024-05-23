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

	groups []*Feature
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
	f.Name = nameFromCaller( /* skip */ 1)
	return f
}

type Option func(f *Feature) error

func WithName(name string) Option {
	return func(f *Feature) error {
		f.Name = name
		return nil
	}
}

func nameFromCaller(skip int) string {
	pc, _, _, _ := runtime.Caller(skip + 1)
	caller := runtime.FuncForPC(pc)
	if caller != nil {
		splitted := strings.Split(caller.Name(), ".")
		return splitted[len(splitted)-1]
	}
	return ""
}

// FeatureSet is a list of features and feature set name.
type FeatureSet struct {
	Name     string
	Features []*Feature
}

// StepFn is the function signature for steps.
type StepFn func(ctx context.Context, t T)

// AsFeature transforms a step function into a feature running the step function at the Setup phase.
func (sf StepFn) AsFeature(options ...Option) *Feature {
	// Default feature configs.
	options = append(options, WithName(nameFromCaller(1)))

	f := NewFeature()

	for _, opt := range options {
		if err := opt(f); err != nil {
			panic(err)
		}
	}

	f.Setup("StepFn", sf)

	return f
}

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
// Expected to be used as a StepFn.
func (f *Feature) DeleteResources(ctx context.Context, t T) {
	if err := DeleteResources(ctx, t, f.References()); err != nil {
		t.Fatal(err)
	}
}

func DeleteResources(ctx context.Context, t T, refs []corev1.ObjectReference) error {
	dc := dynamicclient.Get(ctx)

	for _, ref := range refs {

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return fmt.Errorf("could not parse GroupVersion for %+v", ref.APIVersion)
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

	var lastResource corev1.ObjectReference // One still present resource

	interval, timeout := state.PollTimingsFromContext(ctx)
	err := wait.Poll(interval, timeout, func() (bool, error) {
		for _, ref := range refs {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				return false, fmt.Errorf("could not parse GroupVersion for %+v", ref.APIVersion)
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
				LogReferences(ref)(ctx, t)
				return false, fmt.Errorf("failed to get resource %+v %s/%s: %w", resource, ref.Namespace, ref.Name, err)
			}

			lastResource = ref
			t.Logf("Resource %+v %s/%s still present", resource, ref.Namespace, ref.Name)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		LogReferences(lastResource)(ctx, t)
		return fmt.Errorf("failed to wait for resources to be deleted: %v", err)
	}

	return nil
}

var (
	// Expected to be used as a StepFn.
	_ StepFn = (&Feature{}).DeleteResources
)

// PrerequisiteResult is the result returned by ShouldRun.
type PrerequisiteResult struct {
	// ShouldRun is the flag signaling whether other timings will run or not.
	// True means other timings will run, false will skip other timings.
	ShouldRun bool
	// Reason will report why a given prerequisite is not satisfied.
	// This is used to report a clear skip reason to the user.
	Reason string
}

// ShouldRun is the function signature for Prerequisite steps.
type ShouldRun func(ctx context.Context, t T) (PrerequisiteResult, error)

func (sr ShouldRun) AsStepFn() StepFn {
	return func(ctx context.Context, t T) {
		shouldRun, err := sr(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		if !shouldRun.ShouldRun {
			t.Errorf("Prerequisite: %s", shouldRun.Reason)
		}
	}
}

// Prerequisite adds a step function to the feature set at the Prerequisite timing phase.
func (f *Feature) Prerequisite(name string, fn ShouldRun) {
	f.AddStep(Step{
		Name: name,
		S:    Any,
		L:    All,
		T:    Prerequisite,
		Fn:   fn.AsStepFn(),
	})
}

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

// Group add a new group to the feature, groups are executed in the order they are inserted and
// before the feature steps.
func (f *Feature) Group(name string, group func(f *Feature)) {

	other := &Feature{
		Name:  name,
		State: f.State,
	}

	group(other)

	f.GroupF(other)
}

// GroupF add a new sub Feature to the feature, groups are executed in the order they are inserted
// and before the feature steps.
func (f *Feature) GroupF(other *Feature) {
	f.groups = append(f.groups, other)
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

// GetGroups returns sub-features, this is for rekt internal use only.
func (f *Feature) GetGroups() []*Feature {
	return f.groups
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
