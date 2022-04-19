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

package evenpodspread

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler/factory"
	state "knative.dev/eventing/pkg/scheduler/state"
	"knative.dev/pkg/logging"
)

// EvenPodSpread is a filter or score plugin that picks/favors pods that create an equal spread of resources across pods
type EvenPodSpread struct {
}

// Verify EvenPodSpread Implements FilterPlugin and ScorePlugin Interface
var _ state.FilterPlugin = &EvenPodSpread{}
var _ state.ScorePlugin = &EvenPodSpread{}

// Name of the plugin
const (
	Name                   = state.EvenPodSpread
	ErrReasonInvalidArg    = "invalid arguments"
	ErrReasonUnschedulable = "pod will cause an uneven spread"
)

func init() {
	factory.RegisterFP(Name, &EvenPodSpread{})
	factory.RegisterSP(Name, &EvenPodSpread{})
}

// Name returns name of the plugin
func (pl *EvenPodSpread) Name() string {
	return Name
}

// Filter invoked at the filter extension point.
func (pl *EvenPodSpread) Filter(ctx context.Context, args interface{}, states *state.State, key types.NamespacedName, podID int32) *state.Status {
	logger := logging.FromContext(ctx).With("Filter", pl.Name())

	spreadArgs, ok := args.(string)
	if !ok {
		logger.Errorf("Filter args %v for predicate %q are not valid", args, pl.Name())
		return state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	skewVal := state.EvenPodSpreadArgs{}
	decoder := json.NewDecoder(strings.NewReader(spreadArgs))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&skewVal); err != nil {
		return state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	if states.Replicas > 0 { //need at least a pod to compute spread
		currentReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, podID)] //get #vreps on this podID
		var skew int32
		for _, otherPodID := range states.SchedulablePods { //compare with #vreps on other pods
			if otherPodID != podID {
				otherReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, otherPodID)]

				if otherReps == 0 && states.Free(otherPodID) <= 0 { //other pod fully occupied by other vpods - so ignore
					continue
				}
				if skew = (currentReps + 1) - otherReps; skew < 0 {
					skew = skew * int32(-1)
				}

				//logger.Infof("Current Pod %d with %d and Other Pod %d with %d causing skew %d", podID, currentReps, otherPodID, otherReps, skew)
				if skew > skewVal.MaxSkew {
					logger.Infof("Unschedulable! Pod %d will cause an uneven spread %v with other pod %v", podID, states.PodSpread[key], otherPodID)
					return state.NewStatus(state.Unschedulable, ErrReasonUnschedulable)
				}
			}
		}
	}

	return state.NewStatus(state.Success)
}

// Score invoked at the score extension point. The "score" returned in this function is higher for pods that create an even spread across pods.
func (pl *EvenPodSpread) Score(ctx context.Context, args interface{}, states *state.State, feasiblePods []int32, key types.NamespacedName, podID int32) (uint64, *state.Status) {
	logger := logging.FromContext(ctx).With("Score", pl.Name())
	var score uint64 = 0

	spreadArgs, ok := args.(string)
	if !ok {
		logger.Errorf("Scoring args %v for priority %q are not valid", args, pl.Name())
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	skewVal := state.EvenPodSpreadArgs{}
	decoder := json.NewDecoder(strings.NewReader(spreadArgs))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&skewVal); err != nil {
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	if states.Replicas > 0 { //need at least a pod to compute spread
		currentReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, podID)] //get #vreps on this podID
		var skew int32
		for _, otherPodID := range states.SchedulablePods { //compare with #vreps on other pods
			if otherPodID != podID {
				otherReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, otherPodID)]
				if otherReps == 0 && states.Free(otherPodID) == 0 { //other pod fully occupied by other vpods - so ignore
					continue
				}
				if skew = (currentReps + 1) - otherReps; skew < 0 {
					skew = skew * int32(-1)
				}

				//logger.Infof("Current Pod %d with %d and Other Pod %d with %d causing skew %d", podID, currentReps, otherPodID, otherReps, skew)
				if skew > skewVal.MaxSkew {
					logger.Infof("Pod %d will cause an uneven spread %v with other pod %v", podID, states.PodSpread[key], otherPodID)
				}
				score = score + uint64(skew)
			}
		}
		score = math.MaxUint64 - score //lesser skews get higher score
	}

	return score, state.NewStatus(state.Success)
}

// ScoreExtensions of the Score plugin.
func (pl *EvenPodSpread) ScoreExtensions() state.ScoreExtensions {
	return pl
}

// NormalizeScore invoked after scoring all pods.
func (pl *EvenPodSpread) NormalizeScore(ctx context.Context, states *state.State, scores state.PodScoreList) *state.Status {
	return nil
}
