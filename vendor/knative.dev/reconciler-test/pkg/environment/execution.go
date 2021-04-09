package environment

import (
	"context"
	"sync"
	"testing"

	"knative.dev/reconciler-test/pkg/feature"
)

func categorizeSteps(steps []feature.Step) map[feature.Timing][]feature.Step {
	res := make(map[feature.Timing][]feature.Step, 4)

	res[feature.Setup] = filterStepTimings(steps, feature.Setup)
	res[feature.Requirement] = filterStepTimings(steps, feature.Requirement)
	res[feature.Assert] = filterStepTimings(steps, feature.Assert)
	res[feature.Teardown] = filterStepTimings(steps, feature.Teardown)

	return res
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

// executeWithSkippingT executes the step in a sub test wrapping t in order to never fail the subtest
// This blocks until the test completes.
func (mr *MagicEnvironment) executeWithSkippingT(ctx context.Context, originalT *testing.T, f *feature.Feature, s *feature.Step) feature.T {
	originalT.Helper()
	return mr.executeStep(ctx, originalT, f, s, createSkippingT)
}

// executeWithoutWrappingT executes the step in a sub test without wrapping t.
// This blocks until the test completes.
func (mr *MagicEnvironment) executeWithoutWrappingT(ctx context.Context, originalT *testing.T, f *feature.Feature, s *feature.Step) feature.T {
	originalT.Helper()
	return mr.executeStep(ctx, originalT, f, s, func(t *testing.T) feature.T {
		return t
	})
}

func (mr *MagicEnvironment) executeStep(ctx context.Context, originalT *testing.T, f *feature.Feature, s *feature.Step, tDecorator func(t *testing.T) feature.T) feature.T {
	originalT.Helper()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	var internalT feature.T
	originalT.Run(f.Name+"/"+s.T.String()+"/"+s.TestName(), func(st *testing.T) {
		st.Helper()
		internalT = tDecorator(st)

		// Create a cancel tied to this step
		internalCtx, internalCancelFn := context.WithCancel(ctx)

		defer func() {
			if r := recover(); r != nil {
				internalT.Errorf("Panic happened: '%v'", r)
			}
			// Close the context as soon as possible (this defer is invoked before the Cleanup)
			internalCancelFn()
		}()

		mr.milestones.StepStarted(f.Name, s, internalT)
		internalT.Cleanup(func() {
			mr.milestones.StepFinished(f.Name, s, internalT)
			wg.Done() // Make sure wg.Done() is invoked at the very end of the execution
		})

		// Perform step.
		s.Fn(internalCtx, internalT)
	})

	// Wait for the test to execute before spawning the next one
	wg.Wait()

	return internalT
}
