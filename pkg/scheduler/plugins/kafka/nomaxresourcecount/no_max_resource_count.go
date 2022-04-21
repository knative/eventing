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
	"context"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler/factory"
	state "knative.dev/eventing/pkg/scheduler/state"
	"knative.dev/pkg/logging"
)

// NoMaxResourceCount plugin filters pods that cause total pods with placements to exceed total partitioncount.
type NoMaxResourceCount struct {
}

// Verify NoMaxResourceCount Implements FilterPlugin Interface
var _ state.FilterPlugin = &NoMaxResourceCount{}

// Name of the plugin
const Name = state.NoMaxResourceCount

const (
	ErrReasonInvalidArg    = "invalid arguments"
	ErrReasonUnschedulable = "pod increases total # of pods beyond partition count"
)

func init() {
	factory.RegisterFP(Name, &NoMaxResourceCount{})
}

// Name returns name of the plugin
func (pl *NoMaxResourceCount) Name() string {
	return Name
}

// Filter invoked at the filter extension point.
func (pl *NoMaxResourceCount) Filter(ctx context.Context, args interface{}, states *state.State, key types.NamespacedName, podID int32) *state.Status {
	logger := logging.FromContext(ctx).With("Filter", pl.Name())

	resourceCountArgs, ok := args.(string)
	if !ok {
		logger.Errorf("Filter args %v for predicate %q are not valid", args, pl.Name())
		return state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	resVal := state.NoMaxResourceCountArgs{}
	decoder := json.NewDecoder(strings.NewReader(resourceCountArgs))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&resVal); err != nil {
		return state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	podName := state.PodNameFromOrdinal(states.StatefulSetName, podID)
	if _, ok := states.PodSpread[key][podName]; !ok && ((len(states.PodSpread[key]) + 1) > resVal.NumPartitions) { //pod not in vrep's partition map and counting this new pod towards total pod count
		logger.Infof("Unschedulable! Pod %d filtered due to total pod count %v exceeding partition count", podID, len(states.PodSpread[key])+1)
		return state.NewStatus(state.Unschedulable, ErrReasonUnschedulable)
	}

	return state.NewStatus(state.Success)
}
