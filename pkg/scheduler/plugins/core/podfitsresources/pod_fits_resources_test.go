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

package podfitsresources

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
		podID    int32
		expected *state.Status
		err      error
	}{
		{
			name:     "no vpods",
			state:    &state.State{Capacity: 10, FreeCap: []int32{}, LastOrdinal: -1},
			podID:    0,
			expected: state.NewStatus(state.Success),
		},
		{
			name:     "one vpods free",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(9)}, LastOrdinal: 0},
			podID:    0,
			expected: state.NewStatus(state.Success),
		},
		{
			name:     "one vpods free",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(10)}, LastOrdinal: 0},
			podID:    1,
			expected: state.NewStatus(state.Success),
		},
		{
			name:     "one vpods not free",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(0)}, LastOrdinal: 0},
			podID:    0,
			expected: state.NewStatus(state.Unschedulable, ErrReasonUnschedulable),
		},
		{
			name:     "many vpods, no gaps",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(0), int32(5), int32(5)}, LastOrdinal: 2},
			podID:    0,
			expected: state.NewStatus(state.Unschedulable, ErrReasonUnschedulable),
		},
		{
			name:     "many vpods, with gaps",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(9), int32(10), int32(5), int32(10)}, LastOrdinal: 2},
			podID:    0,
			expected: state.NewStatus(state.Success),
		},
		{
			name:     "many vpods, with gaps and reserved vreplicas",
			state:    &state.State{Capacity: 10, FreeCap: []int32{int32(4), int32(10), int32(5), int32(0)}, LastOrdinal: 2},
			podID:    3,
			expected: state.NewStatus(state.Unschedulable, ErrReasonUnschedulable),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			var plugin = &PodFitsResources{}
			var args interface{}

			name := plugin.Name()
			assert.Equal(t, name, state.PodFitsResources)

			status := plugin.Filter(ctx, args, tc.state, types.NamespacedName{}, tc.podID)
			if !reflect.DeepEqual(status, tc.expected) {
				t.Errorf("unexpected state, got %v, want %v", status, tc.expected)
			}
		})
	}
}
