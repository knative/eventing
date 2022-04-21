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

package lowestordinalpriority

import (
	"context"
	"math"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler/factory"
	state "knative.dev/eventing/pkg/scheduler/state"
)

// LowestOrdinalPriority is a score plugin that favors pods that have a lower ordinal
type LowestOrdinalPriority struct {
}

// Verify LowestOrdinalPriority Implements ScorePlugin Interface
var _ state.ScorePlugin = &LowestOrdinalPriority{}

// Name of the plugin
const Name = state.LowestOrdinalPriority

func init() {
	factory.RegisterSP(Name, &LowestOrdinalPriority{})
}

// Name returns name of the plugin
func (pl *LowestOrdinalPriority) Name() string {
	return Name
}

// Score invoked at the score extension point. The "score" returned in this function is higher for pods with lower ordinal values.
func (pl *LowestOrdinalPriority) Score(ctx context.Context, args interface{}, states *state.State, feasiblePods []int32, key types.NamespacedName, podID int32) (uint64, *state.Status) {
	score := math.MaxUint64 - uint64(podID) //lower ordinals get higher score
	return score, state.NewStatus(state.Success)
}

// ScoreExtensions of the Score plugin.
func (pl *LowestOrdinalPriority) ScoreExtensions() state.ScoreExtensions {
	return pl
}

// NormalizeScore invoked after scoring all pods.
func (pl *LowestOrdinalPriority) NormalizeScore(ctx context.Context, states *state.State, scores state.PodScoreList) *state.Status {
	return nil
}
