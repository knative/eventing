/*
Copyright 2021 The Knative Authors

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

package featured

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/state"
)

var opt = godog.Options{
	Output: colors.Colored(os.Stdout),
}

// Step is a wrapper for a call to godog.ScenarioContext.Step()
type Step struct {
	Expr         string
	StepFuncCtor func(rt interface{}) interface{}
}

func Run(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}

	if len(flag.Args()) > 0 {
		opt.Paths = flag.Args()
	} else {
		opt.Paths = []string{
			filepath.Join(callerPath(), "testdata/features/"),
		}
	}

	format := "progress"
	for _, arg := range os.Args[1:] {
		if arg == "-test.v=true" { // go test transforms -v option
			format = "pretty"
			break
		}
	}

	opt.Format = format

	os.Exit(m.Run())
}

func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {})
}

func TestConformance(t *testing.T, global environment.GlobalEnvironment, customSteps ...Step) {
	status := godog.TestSuite{
		Name:                 "TestConformance",
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			Conformance(t, s, global, customSteps...)
		},
		Options: &opt,
	}.Run()

	if status != 0 {
		t.Fail()
	}
}

func Conformance(t *testing.T, s *godog.ScenarioContext, global environment.GlobalEnvironment, steps ...Step) {
	rt := &ReconcilerTest{
		T:      t,
		global: global,
	}

	s.Step(`^a new environment$`, rt.aNewEnvironment)
	s.Step(`^an existing namespace "([^"]*)" for environment$`, rt.anExistingEnvironment)
	s.Step(`^running test "([^"]*)"$`, rt.runningTest)
	s.Step(`^a new Broker named "([^"]*)" with class "([^"]*)"$`, rt.aNewBrokerNamedWithClass)
	s.Step(`^an existing Broker named "([^"]*)"$`, rt.anExistingBrokerNamed)
	s.Step(`^all PASS$`, func() error { return nil })

	// Inject custom steps.
	for _, step := range steps {
		s.Step(step.Expr, step.StepFuncCtor(rt))
	}

	_ = rt
}

type ReconcilerTest struct {
	T *testing.T

	global environment.GlobalEnvironment
	ctx    context.Context
	env    environment.Environment
}

func (rt *ReconcilerTest) aNewEnvironment() error {
	if rt.env != nil {
		return errors.New("env already configured")
	}

	rt.ctx, rt.env = rt.global.Environment(
		environment.Managed(rt.T),
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	// State for the local set setup.
	rt.ctx = state.ContextWith(rt.ctx, &state.KVStore{})
	return nil
}

func (rt *ReconcilerTest) anExistingEnvironment(namespace string) error {
	if rt.env != nil {
		return errors.New("env already configured")
	}

	rt.ctx, rt.env = rt.global.Environment(
		environment.InNamespace(namespace),
		environment.Managed(rt.T),
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	// State for the local set setup.
	rt.ctx = state.ContextWith(rt.ctx, &state.KVStore{})

	return nil
}

func (rt *ReconcilerTest) runningTest(name string) error {
	if rt.env == nil {
		return errors.New("env not configured")
	}

	switch name {
	case "AsMiddleware":
		name := state.GetStringOrFail(rt.ctx, rt.T, "brokerName")
		rt.env.Test(rt.ctx, rt.T, broker.SourceToSink(name))
	default:
		return fmt.Errorf("unknown test %q", name)
	}

	return nil
}

func (rt *ReconcilerTest) aNewBrokerNamedWithClass(name, class string) error {
	if rt.env == nil {
		return errors.New("env not configured")
	}

	// Install and wait for a Ready Broker.
	rt.env.Prerequisite(rt.ctx, rt.T, broker.BrokerGoesReady(name, class))
	state.SetOrFail(rt.ctx, rt.T, "brokerName", name)

	return nil
}

func (rt *ReconcilerTest) anExistingBrokerNamed(name string) error {
	if rt.env == nil {
		return errors.New("env not configured")
	}

	state.SetOrFail(rt.ctx, rt.T, "brokerName", name)

	return nil
}
