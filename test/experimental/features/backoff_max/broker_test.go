/*
Copyright 2026 The Knative Authors

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

package backoff_max

import (
	"testing"

	"github.com/stretchr/testify/require"
	"knative.dev/reconciler-test/pkg/feature"
)

func TestBrokerToTriggerRunsSenderAfterReadiness(t *testing.T) {
	timings := make(map[string]feature.Timing)
	for _, step := range BrokerToTrigger().Steps {
		timings[step.Name] = step.T
	}

	for _, name := range []string{"broker is ready", "trigger is ready"} {
		timing, ok := timings[name]
		require.Truef(t, ok, "step %q not found", name)
		require.Equal(t, feature.Requirement, timing)
	}

	timing, ok := timings["send event"]
	require.True(t, ok, "step %q not found", "send event")
	require.Equal(t, feature.Assert, timing)
}
