package feature

import (
	"context"
	"testing"
)

// CollapseSteps is used by the framework to impose the execution constraints of the different steps
// Steps with Setup, Requirement and Teardown timings are run sequentially in order
// Steps with Assert timing are run in parallel
func CollapseSteps(steps []Step) []Step {
	var (
		setup, requirement, teardown *Step
		asserts                      []Step
	)

	for i, s := range steps {
		switch s.T {
		case Setup:
			setup = composeStep(setup, &steps[i])
		case Requirement:
			requirement = composeStep(requirement, &steps[i])
		case Assert:
			asserts = append(asserts, parallelizeStep(s))
		case Teardown:
			teardown = composeStep(teardown, &steps[i])
		}
	}

	var result []Step
	if setup != nil {
		result = append(result, *setup)
	}
	if requirement != nil {
		result = append(result, *requirement)
	}
	result = append(result, asserts...)
	if teardown != nil {
		result = append(result, *teardown)
	}

	return result
}

func composeStep(x *Step, y *Step) *Step {
	if x == nil {
		return y
	}
	if y == nil {
		return x
	}
	return &Step{
		Name: x.Name + " and " + y.Name,
		S:    x.S,
		L:    x.L,
		T:    x.T,
		Fn: func(ctx context.Context, t T) {
			// TODO this is just a workaround until we find a proper solution for https://github.com/knative-sandbox/reconciler-test/issues/106
			t.(*testing.T).Helper()
			x.Fn(ctx, t)
			y.Fn(ctx, t)
		},
	}
}

func parallelizeStep(x Step) Step {
	return Step{
		Name: x.Name,
		S:    x.S,
		L:    x.L,
		T:    x.T,
		Fn: func(ctx context.Context, t T) {
			// TODO this is just a workaround until we find a proper solution for https://github.com/knative-sandbox/reconciler-test/issues/106
			t.(*testing.T).Parallel()
			t.(*testing.T).Helper()
			x.Fn(ctx, t)
		},
	}
}
