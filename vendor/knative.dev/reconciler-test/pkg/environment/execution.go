package environment

import (
	"context"
	"sync"
	"testing"

	"knative.dev/reconciler-test/pkg/feature"
)

func categorizeSteps(steps []feature.Step) map[feature.Timing][]feature.Step {
	stepsByTiming := make(map[feature.Timing][]feature.Step, len(feature.Timings()))

	for _, timing := range feature.Timings() {
		stepsByTiming[timing] = filterStepTimings(steps, timing)
	}

	return stepsByTiming
}

func filterStepTimings(steps []feature.Step, timing feature.Timing) []feature.Step {
	var res []feature.Step
	for _, s := range steps {
		if s.T == timing {
			res = append(res, s)
		}
	}
	return res
}

func (mr *MagicEnvironment) executeOptional(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step, aggregator *stepExecutionAggregator) {
	t.Helper()
	mr.executeStep(ctx, t, f, s, createSkippingT, aggregator)
}

// execute executes the step in a sub test without wrapping t.
func (mr *MagicEnvironment) execute(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step, aggregator *stepExecutionAggregator) {
	t.Helper()
	mr.executeStep(ctx, t, f, s, func(t *testing.T) feature.T { return t }, aggregator)
}

func (mr *MagicEnvironment) executeStep(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step, tDecorator func(t *testing.T) feature.T, aggregator *stepExecutionAggregator) {
	t.Helper()

	t.Run(s.Name, func(t *testing.T) {
		t.Parallel()
		t.Helper()
		ft := tDecorator(t)
		t.Cleanup(func() {
			mr.milestones.StepFinished(f.Name, s, ft)
		})

		// Create a cancel tied to this step
		internalCtx, internalCancelFn := context.WithCancel(ctx)

		defer func() {
			if r := recover(); r != nil {
				ft.Errorf("Panic happened: '%v'", r)
			}
			// Close the context as soon as possible (this defer is invoked before the Cleanup)
			internalCancelFn()
		}()

		mr.milestones.StepStarted(f.Name, s, ft)

		// Perform step.
		s.Fn(internalCtx, ft)

		if ft.Failed() {
			aggregator.AddFailed(s)
		} else {
			aggregator.AddSucceeded(s)
		}
	})
}

// stepExecutionAggregator aggregates various parallel steps results.
//
// It needs to be thread safe since steps run in parallel.
type stepExecutionAggregator struct {
	failed  map[string]*feature.Step
	success map[string]*feature.Step
	mu      sync.Mutex
}

func newStepExecutionAggregator() *stepExecutionAggregator {
	return &stepExecutionAggregator{
		failed:  make(map[string]*feature.Step, 2),
		success: make(map[string]*feature.Step, 2),
		mu:      sync.Mutex{},
	}
}

func (er *stepExecutionAggregator) Failed() []*feature.Step {
	er.mu.Lock()
	defer er.mu.Unlock()

	failed := make([]*feature.Step, 0, len(er.failed))
	for _, v := range er.failed {
		failed = append(failed, v)
	}
	return failed
}

func (er *stepExecutionAggregator) AddFailed(step *feature.Step) {
	er.mu.Lock()
	defer er.mu.Unlock()

	er.failed[step.Name] = step
}

func (er *stepExecutionAggregator) AddSucceeded(step *feature.Step) {
	er.mu.Lock()
	defer er.mu.Unlock()

	er.success[step.Name] = step
}
