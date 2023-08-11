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

package state

import (
	"math"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/scheduler"
)

func PodNameFromOrdinal(name string, ordinal int32) string {
	return name + "-" + strconv.Itoa(int(ordinal))
}

func OrdinalFromPodName(podName string) int32 {
	ordinal, err := strconv.ParseInt(podName[strings.LastIndex(podName, "-")+1:], 10, 32)
	if err != nil {
		return math.MaxInt32
	}
	return int32(ordinal)
}

// Get retrieves the VPod from the vpods lister for a given namespace and name.
func GetVPod(key types.NamespacedName, vpods []scheduler.VPod) scheduler.VPod {
	for _, vpod := range vpods {
		if vpod.GetKey().Name == key.Name && vpod.GetKey().Namespace == key.Namespace {
			return vpod
		}
	}
	return nil
}

func SatisfyZoneAvailability(feasiblePods []int32, states *State) bool {
	zoneMap := make(map[string]struct{})
	for _, podID := range feasiblePods {
		zoneName, _, _ := states.GetPodInfo(PodNameFromOrdinal(states.StatefulSetName, podID))
		zoneMap[zoneName] = struct{}{}
	}
	return len(zoneMap) == int(states.NumZones)
}

func SatisfyNodeAvailability(feasiblePods []int32, states *State) bool {
	nodeMap := make(map[string]struct{})
	for _, podID := range feasiblePods {
		_, nodeName, _ := states.GetPodInfo(PodNameFromOrdinal(states.StatefulSetName, podID))
		nodeMap[nodeName] = struct{}{}
	}
	return len(nodeMap) == int(states.NumNodes)
}
