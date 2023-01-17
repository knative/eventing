package environment

import (
	"context"
	"testing"

	"knative.dev/reconciler-test/pkg/feature"
)

func categorizeSteps(steps []feature.Step) map[feature.Timing][]feature.Step {
	stepsByTiming := make(map[feature.Timing][]feature.Step, 4)

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

func (mr *MagicEnvironment) executeOptional(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step) {
	t.Helper()
	mr.executeStep(ctx, t, f, s, createSkippingT)
}

// execute executes the step in a sub test without wrapping t.
func (mr *MagicEnvironment) execute(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step) {
	t.Helper()
	mr.executeStep(ctx, t, f, s, func(t *testing.T) feature.T { return t })
}

func (mr *MagicEnvironment) executeStep(ctx context.Context, t *testing.T, f *feature.Feature, s *feature.Step, tDecorator func(t *testing.T) feature.T) {
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
	})
}
