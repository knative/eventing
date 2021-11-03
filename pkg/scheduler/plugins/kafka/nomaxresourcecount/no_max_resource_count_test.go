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

package nomaxresourcecount

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	state "knative.dev/eventing/pkg/scheduler/state"
	tscheduler "knative.dev/eventing/pkg/scheduler/testing"
)

func TestFilter(t *testing.T) {
	testCases := []struct {
		name     string
		state    *state.State
		vpod     types.NamespacedName
		podID    int32
		expected *state.Status
		args     interface{}
	}{
		{
			name:     "no vpods, no pods",
			vpod:     types.NamespacedName{},
			state:    &state.State{StatefulSetName: "pod-name", LastOrdinal: -1, PodSpread: map[types.NamespacedName]map[string]int32{}},
			podID:    0,
			expected: state.NewStatus(state.Success),
			args:     "{\"NumPartitions\": 5}",
		},
		{
			name:     "no vpods, no pods, bad arg",
			vpod:     types.NamespacedName{},
			state:    &state.State{StatefulSetName: "pod-name", LastOrdinal: -1, PodSpread: map[types.NamespacedName]map[string]int32{}},
			podID:    0,
			expected: state.NewStatus(state.Unschedulable, ErrReasonInvalidArg),
			args:     "{\"NumParts\": 5}",
		},
		{
			name: "one vpod, one pod, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 0,
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
						"pod-name-0": 5,
					},
				},
			},
			podID:    0,
			expected: state.NewStatus(state.Success),
			args:     "{\"NumPartitions\": 5}",
		},
		{
			name: "two vpods, one pod, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 0,
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
						"pod-name-0": 5,
					},
					{Name: "vpod-name-1", Namespace: "vpod-ns-1"}: {
						"pod-name-0": 4,
					},
				},
			},
			podID:    0,
			expected: state.NewStatus(state.Success),
			args:     "{\"NumPartitions\": 5}",
		},
		{
			name: "one vpod, two pods,same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 1, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 5,
					"pod-name-1": 5,
				},
			}},
			podID:    1,
			expected: state.NewStatus(state.Success),
			args:     "{\"NumPartitions\": 5}",
		},
		{
			name: "one vpod, five pods, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 4, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 5,
					"pod-name-1": 4,
					"pod-name-2": 3,
					"pod-name-3": 4,
					"pod-name-4": 5,
				},
			}},
			podID:    1,
			expected: state.NewStatus(state.Success),
			args:     "{\"NumPartitions\": 5}",
		},
		{
			name: "one vpod, five pods, same pod filter unschedulable",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 2, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 7,
					"pod-name-1": 4,
					"pod-name-2": 3,
					"pod-name-3": 4,
					"pod-name-4": 5,
				},
			}},
			podID:    5,
			expected: state.NewStatus(state.Unschedulable, ErrReasonUnschedulable),
			args:     "{\"NumPartitions\": 5}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			var plugin = &NoMaxResourceCount{}

			name := plugin.Name()
			assert.Equal(t, name, state.NoMaxResourceCount)

			status := plugin.Filter(ctx, tc.args, tc.state, tc.vpod, tc.podID)
			if !reflect.DeepEqual(status, tc.expected) {
				t.Errorf("unexpected state, got %v, want %v", status, tc.expected)
			}
		})
	}
}
