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

package environment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/milestone"
	"knative.dev/reconciler-test/pkg/state"
)

func NewGlobalEnvironment(ctx context.Context) GlobalEnvironment {
	fmt.Printf("level %s, state %s\n\n", l, s)

	return &MagicGlobalEnvironment{
		c:                ctx,
		instanceID:       uuid.New().String(),
		RequirementLevel: *l,
		FeatureState:     *s,
	}
}

type MagicGlobalEnvironment struct {
	c context.Context
	// instanceID represents this instance of the GlobalEnvironment. It is used
	// to link runs together from a single global environment.
	instanceID string

	RequirementLevel feature.Levels
	FeatureState     feature.States
}

type MagicEnvironment struct {
	c context.Context
	l feature.Levels
	s feature.States

	images           map[string]string
	namespace        string
	namespaceCreated bool
	refs             []corev1.ObjectReference

	// milestones sends milestone events, if configured.
	milestones milestone.Emitter
}

const (
	NamespaceDeleteErrorReason = "NamespaceDeleteError"
)

func (mr *MagicEnvironment) Reference(ref ...corev1.ObjectReference) {
	mr.refs = append(mr.refs, ref...)
}

func (mr *MagicEnvironment) References() []corev1.ObjectReference {
	return mr.refs
}

func (mr *MagicEnvironment) Finish() {
	if err := mr.DeleteNamespaceIfNeeded(); err != nil {
		mr.milestones.Exception(NamespaceDeleteErrorReason, "failed to delete namespace %q, %v", mr.namespace, err)
		panic(err)
	}
	mr.milestones.Finished()
}

// WithPollTimings is an environment option to override default poll timings.
func WithPollTimings(interval, timeout time.Duration) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		return ContextWithPollTimings(ctx, interval, timeout), nil
	}
}

// Managed enables auto-lifecycle management of the environment. Including:
//  - registers a t.Cleanup callback on env.Finish().
func Managed(t feature.T) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		t.Cleanup(env.Finish)
		return ctx, nil
	}
}

func (mr *MagicGlobalEnvironment) Environment(opts ...EnvOpts) (context.Context, Environment) {
	images, err := ProduceImages()
	if err != nil {
		panic(err)
	}

	namespace := feature.MakeK8sNamePrefix(feature.AppendRandomString("rekt"))

	env := &MagicEnvironment{
		c:         mr.c,
		l:         mr.RequirementLevel,
		s:         mr.FeatureState,
		images:    images,
		namespace: namespace,
	}

	ctx := ContextWith(mr.c, env)

	for _, opt := range opts {
		if ctx, err = opt(ctx, env); err != nil {
			panic(err)
		}
	}

	// It is possible to have milestones set in the options, check for nil in
	// env first before attempting to pull one from the os environment.
	if env.milestones == nil {
		milestones, err := milestone.NewMilestoneEmitterFromEnv(mr.instanceID, namespace)
		if err != nil {
			// This is just an FYI error, don't block the test run.
			logging.FromContext(mr.c).Error("failed to create the milestone event sender", zap.Error(err))
		}
		if milestones != nil {
			env.milestones = milestones
		}
	}

	if err := env.CreateNamespaceIfNeeded(); err != nil {
		panic(err)
	}

	env.milestones.Environment(map[string]string{
		// TODO: we could add more detail here, don't send secrets.
		"requirementLevel": env.RequirementLevel().String(),
		"featureState":     env.FeatureState().String(),
		"namespace":        env.Namespace(),
	})

	return ctx, env
}

func (mr *MagicEnvironment) Images() map[string]string {
	return mr.images
}

func (mr *MagicEnvironment) TemplateConfig(base map[string]interface{}) map[string]interface{} {
	cfg := make(map[string]interface{})
	for k, v := range base {
		cfg[k] = v
	}
	cfg["namespace"] = mr.namespace
	return cfg
}

func (mr *MagicEnvironment) RequirementLevel() feature.Levels {
	return mr.l
}

func (mr *MagicEnvironment) FeatureState() feature.States {
	return mr.s
}

// InNamespace takes the namespace that the tests should be run in instead of creating
// a namespace. This is useful if the Unit Under Test (UUT) is created ahead of the tests
// to be run. Longer term this should make it easier to decouple the UUT from the generic
// conformance tests, for example, create a Broker of type {Kafka, RabbitMQ} and the tests
// themselves should not care.
func InNamespace(namespace string) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		mr, ok := env.(*MagicEnvironment)
		if !ok {
			return ctx, errors.New("InNamespace: not a magic env")
		}
		mr.namespace = namespace
		return ctx, nil
	}
}

func (mr *MagicEnvironment) Namespace() string {
	return mr.namespace
}

func (mr *MagicEnvironment) Prerequisite(ctx context.Context, t *testing.T, f *feature.Feature) {
	t.Helper() // Helper marks the calling function as a test helper function.
	t.Run("Prerequisite", func(t *testing.T) {
		mr.Test(ctx, t, f)
	})
}

// Test implements Environment.Test.
// In the MagicEnvironment implementation, the Store that is inside of the
// Feature will be assigned to the context. If no Store is set on Feature,
// Test will create a new store.KVStore and set it on the feature and then
// apply it to the Context.
func (mr *MagicEnvironment) Test(ctx context.Context, originalT *testing.T, f *feature.Feature) {
	originalT.Helper() // Helper marks the calling function as a test helper function.

	mr.milestones.TestStarted(f.Name, originalT)
	originalT.Cleanup(func() {
		mr.milestones.TestFinished(f.Name, originalT)
	})

	if f.State == nil {
		f.State = &state.KVStore{}
	}
	ctx = state.ContextWith(ctx, f.State)

	steps := categorizeSteps(f.Steps)

	skipAssertions := false
	skipRequirements := false
	skipReason := ""

	for _, s := range steps[feature.Setup] {
		s := s

		// Setup are executed always, no matter their level and state
		internalT := mr.executeWithoutWrappingT(ctx, originalT, f, &s)

		// Failed setup fails everything, so just run the teardown
		if internalT.Failed() {
			skipAssertions = true
			skipRequirements = true // No need to test other requirements
		}
	}

	for _, s := range steps[feature.Requirement] {
		s := s

		if skipRequirements {
			break
		}

		// Requirement never fails the parent test
		internalT := mr.executeWithSkippingT(ctx, originalT, f, &s)

		if internalT.Failed() {
			skipAssertions = true
			skipRequirements = true // No need to test other requirements
			skipReason = fmt.Sprintf("requirement %q failed", s.Name)
		}
	}

	for _, s := range steps[feature.Assert] {
		s := s

		if skipAssertions {
			break
		}

		if mr.shouldFail(&s) {
			mr.executeWithoutWrappingT(ctx, originalT, f, &s)
		} else {
			mr.executeWithSkippingT(ctx, originalT, f, &s)
		}

		// TODO implement fail fast feature to avoid proceeding with testing if an "expected level" assert fails here
	}

	for _, s := range steps[feature.Teardown] {
		s := s

		wg := &sync.WaitGroup{}
		wg.Add(1)

		// Teardown are executed always, no matter their level and state
		mr.executeWithoutWrappingT(ctx, originalT, f, &s)
	}

	if skipReason != "" {
		originalT.Skipf("Skipping feature '%s' assertions because %s", f.Name, skipReason)
	}
}

func (mr *MagicEnvironment) shouldFail(s *feature.Step) bool {
	return !(mr.s&s.S == 0 || mr.l&s.L == 0)
}

// TestSet implements Environment.TestSet
func (mr *MagicEnvironment) TestSet(ctx context.Context, t *testing.T, fs *feature.FeatureSet) {
	t.Helper() // Helper marks the calling function as a test helper function

	mr.milestones.TestSetStarted(fs.Name, t)
	t.Cleanup(func() {
		mr.milestones.TestSetFinished(fs.Name, t)
	})

	wg := &sync.WaitGroup{}
	for _, f := range fs.Features {
		wg.Add(1)
		t.Run(fs.Name, func(t *testing.T) {
			t.Cleanup(wg.Done)
			// FeatureSets should be run in parallel.
			mr.Test(ctx, t, &f)
		})
	}

	wg.Wait()
}

type envKey struct{}

func ContextWith(ctx context.Context, e Environment) context.Context {
	return context.WithValue(ctx, envKey{}, e)
}

func FromContext(ctx context.Context) Environment {
	if e, ok := ctx.Value(envKey{}).(Environment); ok {
		return e
	}
	panic("no Environment found in context")
}
