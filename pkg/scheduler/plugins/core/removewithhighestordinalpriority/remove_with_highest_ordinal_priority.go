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

package removewithhighestordinalpriority

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler/factory"
	state "knative.dev/eventing/pkg/scheduler/state"
)

// RemoveWithHighestOrdinalPriority is a score plugin that favors pods that have a higher ordinal
type RemoveWithHighestOrdinalPriority struct {
}

// Verify RemoveWithHighestOrdinalPriority Implements ScorePlugin Interface
var _ state.ScorePlugin = &RemoveWithHighestOrdinalPriority{}

// Name of the plugin
const Name = state.RemoveWithHighestOrdinalPriority

func init() {
	factory.RegisterSP(Name, &RemoveWithHighestOrdinalPriority{})
}

// Name returns name of the plugin
func (pl *RemoveWithHighestOrdinalPriority) Name() string {
	return Name
}

// Score invoked at the score extension point. The "score" returned in this function is higher for pods with higher ordinal values.
func (pl *RemoveWithHighestOrdinalPriority) Score(ctx context.Context, args interface{}, states *state.State, feasiblePods []int32, key types.NamespacedName, podID int32) (uint64, *state.Status) {
	score := uint64(podID) //higher ordinals get higher score
	return score, state.NewStatus(state.Success)
}

// ScoreExtensions of the Score plugin.
func (pl *RemoveWithHighestOrdinalPriority) ScoreExtensions() state.ScoreExtensions {
	return pl
}

// NormalizeScore invoked after scoring all pods.
func (pl *RemoveWithHighestOrdinalPriority) NormalizeScore(ctx context.Context, states *state.State, scores state.PodScoreList) *state.Status {
	return nil
}
