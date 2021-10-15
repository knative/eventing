/*
Copyright 2020 The Knative Authors

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

package scheduler

import (
	"k8s.io/apimachinery/pkg/util/sets"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
)

// GetTotalVReplicas returns the total number of placed virtual replicas
func GetTotalVReplicas(placements []duckv1alpha1.Placement) int32 {
	r := int32(0)
	for _, p := range placements {
		r += p.VReplicas
	}
	return r
}

// GetPlacementForPod returns the placement corresponding to podName
func GetPlacementForPod(placements []duckv1alpha1.Placement, podName string) *duckv1alpha1.Placement {
	for i := 0; i < len(placements); i++ {
		if placements[i].PodName == podName {
			return &placements[i]
		}
	}
	return nil
}

// GetPodCount returns the number of pods with the given placements
func GetPodCount(placements []duckv1alpha1.Placement) int {
	set := sets.NewString()
	for _, p := range placements {
		if p.VReplicas > 0 {
			set.Insert(p.PodName)
		}
	}
	return set.Len()
}
