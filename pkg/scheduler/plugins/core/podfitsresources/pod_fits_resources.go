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
	"context"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler/factory"
	state "knative.dev/eventing/pkg/scheduler/state"
	"knative.dev/pkg/logging"
)

// PodFitsResources is a plugin that filters pods that do not have sufficient free capacity for a vreplica to be placed on it
type PodFitsResources struct {
}

// Verify PodFitsResources Implements FilterPlugin Interface
var _ state.FilterPlugin = &PodFitsResources{}

// Name of the plugin
const Name = state.PodFitsResources

const (
	ErrReasonUnschedulable = "pod at full capacity"
)

func init() {
	factory.RegisterFP(Name, &PodFitsResources{})
}

// Name returns name of the plugin
func (pl *PodFitsResources) Name() string {
	return Name
}

// Filter invoked at the filter extension point.
func (pl *PodFitsResources) Filter(ctx context.Context, args interface{}, states *state.State, key types.NamespacedName, podID int32) *state.Status {
	logger := logging.FromContext(ctx).With("Filter", pl.Name())

	if len(states.FreeCap) == 0 || states.Free(podID) > 0 { //vpods with no placements or pods with positive free cap
		return state.NewStatus(state.Success)
	}

	logger.Infof("Unschedulable! Pod %d has no free capacity %v", podID, states.FreeCap)
	return state.NewStatus(state.Unschedulable, ErrReasonUnschedulable)
}
