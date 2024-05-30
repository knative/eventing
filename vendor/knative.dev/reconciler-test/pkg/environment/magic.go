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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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

// NewGlobalEnvironment creates a new global environment based on a
// context.Context, and optional initializers slice. The provided context is
// expected to contain the configured Kube client already.
func NewGlobalEnvironment(ctx context.Context, initializers ...func()) GlobalEnvironment {
	return &MagicGlobalEnvironment{
		RequirementLevel: *l,
		FeatureState:     *s,
		FeatureMatch:     regexp.MustCompile(*f),
		c:                initializeImageStores(ctx),
		instanceID:       uuid.New().String(),
		initializers:     initializers,
		teardownOnFail:   *teardownOnFail,
	}
}

type MagicGlobalEnvironment struct {
	RequirementLevel feature.Levels
	FeatureState     feature.States
	FeatureMatch     *regexp.Regexp

	c context.Context
	// instanceID represents this instance of the GlobalEnvironment. It is used
	// to link runs together from a single global environment.
	instanceID       string
	initializers     []func()
	initializersOnce sync.Once
	teardownOnFail   bool
}

type MagicEnvironment struct {
	c            context.Context
	l            feature.Levels
	s            feature.States
	featureMatch *regexp.Regexp

	namespace        string
	namespaceCreated bool
	refs             []corev1.ObjectReference
	refsMu           sync.Mutex

	// milestones sends milestone events, if configured.
	milestones milestone.Emitter

	// managedT is used for test-scoped logging, if configured.
	managedT feature.T

	// imagePullSecretNamespace/imagePullSecretName: An optional secret to add to service account of new namespaces
	imagePullSecretName      string
	imagePullSecretNamespace string

	teardownOnFail bool
}

var (
	_ Environment = &MagicEnvironment{}
)

const (
	NamespaceDeleteErrorReason = "NamespaceDeleteError"
)

type parallelKey struct{}

func withParallel(ctx context.Context) context.Context {
	return context.WithValue(ctx, parallelKey{}, true)
}

func isParallel(ctx context.Context) bool {
	v := ctx.Value(parallelKey{})
	return v != nil && v.(bool)
}

func (mr *MagicEnvironment) Reference(ref ...corev1.ObjectReference) {
	mr.refsMu.Lock()
	defer mr.refsMu.Unlock()

	mr.refs = append(mr.refs, ref...)
}

func (mr *MagicEnvironment) References() []corev1.ObjectReference {
	mr.refsMu.Lock()
	defer mr.refsMu.Unlock()

	r := make([]corev1.ObjectReference, len(mr.refs))
	copy(r, mr.refs)
	return r
}

func (mr *MagicEnvironment) Finish() {
	// Delete the namespace after sending the Finished milestone event
	// since emitters might use the namespace.
	var result milestone.Result = unknownResult{}
	if mr.managedT != nil {
		result = mr.managedT
		if !result.Failed() {
			if err := feature.DeleteResources(mr.c, mr.managedT, mr.References()); err != nil {
				mr.managedT.Fatal(err)
			}
		}
	}
	if mr.milestones != nil {
		mr.milestones.Finished(result)
	}
	if err := mr.DeleteNamespaceIfNeeded(result); err != nil {
		if mr.milestones != nil {
			mr.milestones.Exception(NamespaceDeleteErrorReason,
				"failed to delete namespace %q, %v", mr.namespace, err)
		}
		panic(err)
	}
}

// WithPollTimings is an environment option to override default poll timings.
func WithPollTimings(interval, timeout time.Duration) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		return ContextWithPollTimings(ctx, interval, timeout), nil
	}
}

// Managed enables auto-lifecycle management of the environment. Including
// registration of following opts:
//   - Cleanup,
//   - WithTestLogger.
func Managed(t feature.T) EnvOpts {
	return UnionOpts(Cleanup(t), WithTestLogger(t))
}

// Cleanup is an environment option to register a cleanup that will call
// Environment.Finish function at test end automatically.
func Cleanup(t feature.T) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		if e, ok := env.(*MagicEnvironment); ok {
			e.managedT = t
		}
		t.Cleanup(env.Finish)
		return ctx, nil
	}
}

func WithEmitter(emitter milestone.Emitter) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		if e, ok := env.(*MagicEnvironment); ok {
			e.milestones = emitter
		}
		return ctx, nil
	}
}

func (mr *MagicGlobalEnvironment) Environment(opts ...EnvOpts) (context.Context, Environment) {
	opts = append(opts, inNamespace())

	env := &MagicEnvironment{
		c:              mr.c,
		l:              mr.RequirementLevel,
		s:              mr.FeatureState,
		featureMatch:   mr.FeatureMatch,
		teardownOnFail: mr.teardownOnFail,

		imagePullSecretName:      "kn-test-image-pull-secret",
		imagePullSecretNamespace: "default",
	}

	ctx := ContextWith(mr.c, env)
	ctx = ContextWithPollTimings(ctx, *pollInterval, *pollTimeout)

	for _, opt := range opts {
		if nctx, err := opt(ctx, env); err != nil {
			logging.FromContext(ctx).Fatal(err)
		} else {
			ctx = nctx
		}
	}
	env.c = ctx

	log := logging.FromContext(ctx)
	log.Infof("Environment settings: level %s, state %s, feature %q",
		env.l, env.s, env.featureMatch)
	mr.initializersOnce.Do(func() {
		for _, initializer := range mr.initializers {
			initializer()
		}
	})

	eventEmitter, err := milestone.NewMilestoneEmitterFromEnv(mr.instanceID, env.namespace)
	if err != nil {
		// This is just an FYI error, don't block the test run.
		logging.FromContext(ctx).Error("failed to create the milestone event sender", zap.Error(err))
	}
	logEmitter := milestone.NewLogEmitter(ctx, env.namespace)

	if env.milestones == nil {
		env.milestones = milestone.Compose(eventEmitter, logEmitter)
	} else {
		// Compose the emitters with those passed through opts.
		env.milestones = milestone.Compose(env.milestones, eventEmitter, logEmitter)
	}

	if err := env.CreateNamespaceIfNeeded(); err != nil {
		logging.FromContext(ctx).Fatal(err)
	}

	for _, in := range GetPostInit(ctx) {
		ctx, err = in(ctx, env)
		if err != nil {
			logging.FromContext(ctx).Fatal(err)
		}
	}

	env.milestones.Environment(map[string]string{
		// TODO: we could add more detail here, don't send secrets.
		"requirementLevel": env.RequirementLevel().String(),
		"featureState":     env.FeatureState().String(),
		"namespace":        env.Namespace(),
	})

	return ctx, env
}

type postInitKey struct{}

type InitFn = EnvOpts

func WithPostInit(ctx context.Context, fn InitFn) context.Context {
	fns := GetPostInit(ctx)
	fns = append(fns, fn)
	return context.WithValue(ctx, postInitKey{}, fns)
}

func GetPostInit(ctx context.Context) []InitFn {
	fns := ctx.Value(postInitKey{})
	if fns == nil {
		return []InitFn{}
	}
	return fns.([]InitFn)
}

func inNamespace() EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		ns := getNamespace(ctx)
		if ns == "" {
			ns = feature.MakeK8sNamePrefix(feature.AppendRandomString("test"))
		}
		return InNamespace(ns)(ctx, env)
	}
}

func (mr *MagicEnvironment) TemplateConfig(base map[string]interface{}) map[string]interface{} {
	cfg := make(map[string]interface{})
	for k, v := range base {
		cfg[k] = v
	}
	if _, ok := cfg["namespace"]; !ok {
		cfg["namespace"] = mr.namespace
	}
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

// WithImagePullSecret takes namespace and name of a Secret to be added to ServiceAccounts
// of newly created namespaces. This is useful if tests refer to private registries that require
// authentication.
func WithImagePullSecret(namespace string, name string) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		mr, ok := env.(*MagicEnvironment)
		if !ok {
			return ctx, errors.New("WithImagePullSecret: not a magic env")
		}
		mr.imagePullSecretNamespace = namespace
		mr.imagePullSecretName = name
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
	mr.test(ctx, originalT, f)
}

// ParallelTest implements Environment.ParallelTest.
// It is similar to Test with the addition of running the feature in parallel
func (mr *MagicEnvironment) ParallelTest(ctx context.Context, originalT *testing.T, f *feature.Feature) {
	mr.test(withParallel(ctx), originalT, f)
}

// Test implements Environment.Test.
// In the MagicEnvironment implementation, the Store that is inside of the
// Feature will be assigned to the context. If no Store is set on Feature,
// Test will create a new store.KVStore and set it on the feature and then
// apply it to the Context.
func (mr *MagicEnvironment) test(ctx context.Context, originalT *testing.T, f *feature.Feature) {
	originalT.Helper() // Helper marks the calling function as a test helper function.

	for i, g := range f.GetGroups() {
		originalT.Run(fmt.Sprintf("group-%d", i+1), func(t *testing.T) {
			mr.test(ctx, t, g)
		})
		if originalT.Failed() { // If a group fails, return
			return
		}
	}

	log := logging.FromContext(ctx)
	f.DumpWith(log.Debug)
	defer f.DumpWith(log.Debug) // Log feature state at the end of the run

	if !mr.featureMatch.MatchString(f.Name) {
		log.Warnf("Skipping feature '%s' assertions because --feature=%s  doesn't match", f.Name, mr.featureMatch.String())
		return
	}

	mr.milestones.TestStarted(f.Name, originalT)
	defer mr.milestones.TestFinished(f.Name, originalT)

	if f.State == nil {
		f.State = &state.KVStore{}
	}
	ctx = state.ContextWith(ctx, f.State)
	ctx = feature.ContextWith(ctx, f)

	stepsByTiming := categorizeSteps(f.Steps)

	mr.milestones.StepsPlanned(f.Name, stepsByTiming, originalT)

	// skip is flag that signals whether the steps for the subsequent timings should
	// be skipped because a step in a previous timing failed.
	//
	// Setup and Teardown steps are executed always except when a Prerequisite step failed.
	skip := false
	skipTeardown := false

	originalT.Run(f.Name, func(t *testing.T) {

		if isParallel(ctx) {
			t.Parallel()
		}

		for _, timing := range feature.Timings() {
			steps := feature.Steps(stepsByTiming[timing])

			// Special case for teardown timing
			if timing == feature.Teardown {
				if skip {
					if mr.teardownOnFail {
						// Prepend logging steps to the teardown phase when a previous timing failed.
						steps = append(mr.loggingSteps(), steps...)
					} else {
						// When not doing teardown only execute logging steps.
						steps = mr.loggingSteps()
					}
				}
				skip = skipTeardown
			}

			originalT.Logf("Running %d steps for timing:\n%s\n\n", len(steps), steps.String())

			// aggregator aggregates steps results (success or failure) for a single timing.
			// It used for handling the prerequisite logic.
			aggregator := newStepExecutionAggregator()

			t.Run(timing.String(), func(t *testing.T) {
				// no parallel, various timing steps run in order: setup, requirement, assert, teardown

				if skip {
					t.Skipf("Skipping steps for timing %s due to failed previous timing\n", timing.String())
					return
				}

				for _, s := range steps {
					s := s
					if mr.shouldFail(&s) && timing != feature.Prerequisite {
						mr.execute(ctx, t, f, &s, aggregator)
					} else {
						mr.executeOptional(ctx, t, f, &s, aggregator)
					}
				}
			})

			// If any step at timing feature.Prerequisite failed, we should skip the feature.
			if timing == feature.Prerequisite {
				failed := aggregator.Failed()
				if len(failed) > 0 {
					bytes, _ := json.MarshalIndent(failed, "", "  ")
					originalT.Logf("Prerequisite steps failed, skipping the remaining timings and steps, failed prerequisite steps:\n%s\n", string(bytes))

					// Skip any other subsequent timing.
					skip = true
					skipTeardown = true
				}
			}

			if t.Failed() {
				// skip the following timings since curring timing failed
				skip = true
			}
		}
	})
}

type unknownResult struct{}

func (u unknownResult) Failed() bool {
	return false
}

// TODO: this logic is strange and hard to follow.
func (mr *MagicEnvironment) shouldFail(s *feature.Step) bool {
	// if it's _not_ alpha nor must
	return !(mr.s&s.S == 0 || mr.l&s.L == 0)
}

// TestSet implements Environment.TestSet
func (mr *MagicEnvironment) TestSet(ctx context.Context, t *testing.T, fs *feature.FeatureSet) {
	mr.testSet(ctx, t, fs)
}

// ParallelTestSet implements Environment.ParallelTestSet
func (mr *MagicEnvironment) ParallelTestSet(ctx context.Context, t *testing.T, fs *feature.FeatureSet) {
	mr.testSet(withParallel(ctx), t, fs)
}

func (mr *MagicEnvironment) testSet(ctx context.Context, t *testing.T, fs *feature.FeatureSet) {
	t.Helper() // Helper marks the calling function as a test helper function

	mr.milestones.TestSetStarted(fs.Name, t)
	defer mr.milestones.TestSetFinished(fs.Name, t)

	for _, f := range fs.Features {
		// Make sure the name is appended
		f.Name = fs.Name + "/" + f.Name
		mr.Test(ctx, t, f)
	}
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
