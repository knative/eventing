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
	"fmt"
	"runtime"
	"strings"

	"knative.dev/reconciler-test/pkg/state"
)

// Feature is a list of steps and feature name.
type Feature struct {
	Name  string
	Steps []Step
	State state.Store
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
	Features []Feature
}

// StepFn is the function signature for steps.
type StepFn func(ctx context.Context, t T)

// Step is a structure to hold the step function, step name and state, level and
// timing configuration.
type Step struct {
	Name string
	S    States
	L    Levels
	T    Timing
	Fn   StepFn
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
