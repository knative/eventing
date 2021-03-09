package environment

import "knative.dev/reconciler-test/pkg/feature"

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
