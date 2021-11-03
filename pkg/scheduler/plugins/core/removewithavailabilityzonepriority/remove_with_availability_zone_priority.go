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

package removewithavailabilityzonepriority

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

// RemoveWithAvailabilityZonePriority is a score plugin that favors pods that create an even spread of resources across zones for HA
type RemoveWithAvailabilityZonePriority struct {
}

// Verify RemoveWithAvailabilityZonePriority Implements ScorePlugin Interface
var _ state.ScorePlugin = &RemoveWithAvailabilityZonePriority{}

// Name of the plugin
const Name = state.RemoveWithAvailabilityZonePriority

const (
	ErrReasonInvalidArg = "invalid arguments"
	ErrReasonNoResource = "zone does not exist"
)

func init() {
	factory.RegisterSP(Name, &RemoveWithAvailabilityZonePriority{})
}

// Name returns name of the plugin
func (pl *RemoveWithAvailabilityZonePriority) Name() string {
	return Name
}

// Score invoked at the score extension point. The "score" returned in this function is higher for zones that create an even spread across zones.
func (pl *RemoveWithAvailabilityZonePriority) Score(ctx context.Context, args interface{}, states *state.State, feasiblePods []int32, key types.NamespacedName, podID int32) (uint64, *state.Status) {
	logger := logging.FromContext(ctx).With("Score", pl.Name())
	var score uint64 = 0

	spreadArgs, ok := args.(string)
	if !ok {
		logger.Errorf("Scoring args %v for priority %q are not valid", args, pl.Name())
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	skewVal := state.AvailabilityZonePriorityArgs{}
	decoder := json.NewDecoder(strings.NewReader(spreadArgs))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&skewVal); err != nil {
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	if states.Replicas > 0 { //need at least a pod to compute spread
		var skew int32
		zoneMap := make(map[string]struct{})
		for _, zoneName := range states.NodeToZoneMap {
			zoneMap[zoneName] = struct{}{}
		}

		zoneName, _, err := states.GetPodInfo(state.PodNameFromOrdinal(states.StatefulSetName, podID))
		if err != nil {
			return score, state.NewStatus(state.Error, ErrReasonNoResource)
		}

		currentReps := states.ZoneSpread[key][zoneName] //get #vreps on this zone
		for otherZoneName := range zoneMap {            //compare with #vreps on other pods
			if otherZoneName != zoneName {
				otherReps, ok := states.ZoneSpread[key][otherZoneName]
				if !ok {
					continue //zone does not exist in current placement, so move on
				}
				if skew = (currentReps - 1) - otherReps; skew < 0 {
					skew = skew * int32(-1)
				}

				//logger.Infof("Current Zone %v with %d and Other Zone %v with %d causing skew %d", zoneName, currentReps, otherZoneName, otherReps, skew)
				if skew > skewVal.MaxSkew { //score low
					logger.Infof("Pod %d in zone %v will cause an uneven zone spread %v with other zone %v", podID, zoneName, states.ZoneSpread[key], otherZoneName)
				}
				score = score + uint64(skew)
			}
		}

		score = math.MaxUint64 - score //lesser skews get higher score
	}

	return score, state.NewStatus(state.Success)
}

// ScoreExtensions of the Score plugin.
func (pl *RemoveWithAvailabilityZonePriority) ScoreExtensions() state.ScoreExtensions {
	return pl
}

// NormalizeScore invoked after scoring all pods.
func (pl *RemoveWithAvailabilityZonePriority) NormalizeScore(ctx context.Context, states *state.State, scores state.PodScoreList) *state.Status {
	return nil
}
