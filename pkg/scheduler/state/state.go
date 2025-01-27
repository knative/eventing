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
	"context"
	"encoding/json"
	"math"
	"strconv"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/scheduler"
)

type StateAccessor interface {
	// State returns the current state (snapshot) about placed vpods
	// Take into account reserved vreplicas and update `reserved` to reflect
	// the current state.
	State(ctx context.Context) (*State, error)
}

// state provides information about the current scheduling of all vpods
// It is used by for the scheduler and the autoscaler
type State struct {
	// free tracks the free capacity of each pod.
	//
	// Including pods that might not exist anymore, it reflects the free capacity determined by
	// placements in the vpod status.
	FreeCap []int32

	// schedulable pods tracks the pods that aren't being evicted.
	SchedulablePods []int32

	// Pod capacity.
	Capacity int32

	// Replicas is the (cached) number of statefulset replicas.
	Replicas int32

	StatefulSetName string

	PodLister corev1.PodNamespaceLister

	// Stores for each vpod, a map of podname to number of vreplicas placed on that pod currently
	PodSpread map[types.NamespacedName]map[string]int32

	// Pending tracks the number of virtual replicas that haven't been scheduled yet
	// because there wasn't enough free capacity.
	Pending map[types.NamespacedName]int32

	// ExpectedVReplicaByVPod is the expected virtual replicas for each vpod key
	ExpectedVReplicaByVPod map[types.NamespacedName]int32
}

// Free safely returns the free capacity at the given ordinal
func (s *State) Free(ordinal int32) int32 {
	if int32(len(s.FreeCap)) <= ordinal {
		return s.Capacity
	}
	return s.FreeCap[ordinal]
}

// SetFree safely sets the free capacity at the given ordinal
func (s *State) SetFree(ordinal int32, value int32) {
	s.FreeCap = grow(s.FreeCap, ordinal, s.Capacity)
	s.FreeCap[int(ordinal)] = value
}

// FreeCapacity returns the number of vreplicas that can be used,
// up to the last ordinal
func (s *State) FreeCapacity() int32 {
	t := int32(0)
	for _, i := range s.SchedulablePods {
		t += s.FreeCap[i]
	}
	return t
}

func (s *State) IsSchedulablePod(ordinal int32) bool {
	for _, x := range s.SchedulablePods {
		if x == ordinal {
			return true
		}
	}
	return false
}

// stateBuilder reconstruct the state from scratch, by listing vpods
type stateBuilder struct {
	vpodLister       scheduler.VPodLister
	capacity         int32
	statefulSetCache *scheduler.ScaleCache
	statefulSetName  string
	podLister        corev1.PodNamespaceLister
}

// NewStateBuilder returns a StateAccessor recreating the state from scratch each time it is requested
func NewStateBuilder(sfsname string, lister scheduler.VPodLister, podCapacity int32, podlister corev1.PodNamespaceLister, statefulSetCache *scheduler.ScaleCache) StateAccessor {

	return &stateBuilder{
		vpodLister:       lister,
		capacity:         podCapacity,
		statefulSetCache: statefulSetCache,
		statefulSetName:  sfsname,
		podLister:        podlister,
	}
}

func (s *stateBuilder) State(ctx context.Context) (*State, error) {
	vpods, err := s.vpodLister()
	if err != nil {
		return nil, err
	}

	logger := logging.FromContext(ctx).With("subcomponent", "statebuilder")
	ctx = logging.WithLogger(ctx, logger)

	scale, err := s.statefulSetCache.GetScale(ctx, s.statefulSetName, metav1.GetOptions{})
	if err != nil {
		logger.Infow("failed to get statefulset", zap.Error(err))
		return nil, err
	}

	freeCap := make([]int32, 0)
	pending := make(map[types.NamespacedName]int32, 4)
	expectedVReplicasByVPod := make(map[types.NamespacedName]int32, len(vpods))
	schedulablePods := sets.NewInt32()

	podSpread := make(map[types.NamespacedName]map[string]int32)

	for podId := int32(0); podId < scale.Spec.Replicas && s.podLister != nil; podId++ {
		pod, err := s.podLister.Get(PodNameFromOrdinal(s.statefulSetName, podId))
		if err != nil {
			logger.Warnw("Failed to get pod", zap.Int32("ordinal", podId), zap.Error(err))
			continue
		}
		if isPodUnschedulable(pod) {
			// Pod is marked for eviction - CANNOT SCHEDULE VREPS on this pod.
			logger.Debugw("Pod is unschedulable", zap.Any("pod", pod))
			continue
		}

		// Pod has no annotation or not annotated as unschedulable and
		// not on an unschedulable node, so add to feasible
		schedulablePods.Insert(podId)
	}

	for _, p := range schedulablePods.List() {
		freeCap = s.updateFreeCapacity(logger, freeCap, PodNameFromOrdinal(s.statefulSetName, p), 0)
	}

	// Getting current state from existing placements for all vpods
	for _, vpod := range vpods {
		ps := vpod.GetPlacements()

		pending[vpod.GetKey()] = pendingFromVPod(vpod)
		expectedVReplicasByVPod[vpod.GetKey()] = vpod.GetVReplicas()

		podSpread[vpod.GetKey()] = make(map[string]int32)

		for i := 0; i < len(ps); i++ {
			podName := ps[i].PodName
			vreplicas := ps[i].VReplicas

			freeCap = s.updateFreeCapacity(logger, freeCap, podName, vreplicas)

			pod, err := s.podLister.Get(podName)
			if err != nil {
				logger.Warnw("Failed to get pod", zap.String("podName", podName), zap.Error(err))
			}

			if pod != nil && schedulablePods.Has(OrdinalFromPodName(pod.GetName())) {
				podSpread[vpod.GetKey()][podName] = podSpread[vpod.GetKey()][podName] + vreplicas
			}
		}
	}

	state := &State{
		FreeCap:                freeCap,
		SchedulablePods:        schedulablePods.List(),
		Capacity:               s.capacity,
		Replicas:               scale.Spec.Replicas,
		StatefulSetName:        s.statefulSetName,
		PodLister:              s.podLister,
		PodSpread:              podSpread,
		Pending:                pending,
		ExpectedVReplicaByVPod: expectedVReplicasByVPod,
	}

	logger.Infow("cluster state info", zap.Any("state", state))

	return state, nil
}

func pendingFromVPod(vpod scheduler.VPod) int32 {
	expected := vpod.GetVReplicas()
	scheduled := scheduler.GetTotalVReplicas(vpod.GetPlacements())

	return int32(math.Max(float64(0), float64(expected-scheduled)))
}

func (s *stateBuilder) updateFreeCapacity(logger *zap.SugaredLogger, freeCap []int32, podName string, vreplicas int32) []int32 {
	ordinal := OrdinalFromPodName(podName)
	freeCap = grow(freeCap, ordinal, s.capacity)

	freeCap[ordinal] -= vreplicas

	// Assert the pod is not overcommitted
	if overcommit := freeCap[ordinal]; overcommit < 0 {
		// This should not happen anymore. Log as an error but do not interrupt the current scheduling.
		logger.Warnw("pod is overcommitted", zap.String("podName", podName), zap.Int32("overcommit", overcommit))
	}

	return freeCap
}

func (s *State) TotalPending() int32 {
	t := int32(0)
	for _, p := range s.Pending {
		t += p
	}
	return t
}

func (s *State) TotalExpectedVReplicas() int32 {
	t := int32(0)
	for _, v := range s.ExpectedVReplicaByVPod {
		t += v
	}
	return t
}

func grow(slice []int32, ordinal int32, def int32) []int32 {
	l := int32(len(slice))
	diff := ordinal - l + 1

	if diff <= 0 {
		return slice
	}

	for i := int32(0); i < diff; i++ {
		slice = append(slice, def)
	}
	return slice
}

func isPodUnschedulable(pod *v1.Pod) bool {
	annotVal, ok := pod.ObjectMeta.Annotations[scheduler.PodAnnotationKey]
	unschedulable, err := strconv.ParseBool(annotVal)

	isMarkedUnschedulable := ok && err == nil && unschedulable
	isPending := pod.Spec.NodeName == ""

	return isMarkedUnschedulable || isPending
}

func (s *State) MarshalJSON() ([]byte, error) {

	type S struct {
		FreeCap         []int32                     `json:"freeCap"`
		SchedulablePods []int32                     `json:"schedulablePods"`
		Capacity        int32                       `json:"capacity"`
		Replicas        int32                       `json:"replicas"`
		StatefulSetName string                      `json:"statefulSetName"`
		PodSpread       map[string]map[string]int32 `json:"podSpread"`
		Pending         map[string]int32            `json:"pending"`
	}

	sj := S{
		FreeCap:         s.FreeCap,
		SchedulablePods: s.SchedulablePods,
		Capacity:        s.Capacity,
		Replicas:        s.Replicas,
		StatefulSetName: s.StatefulSetName,
		PodSpread:       ToJSONable(s.PodSpread),
		Pending:         toJSONablePending(s.Pending),
	}

	return json.Marshal(sj)
}

func ToJSONable(ps map[types.NamespacedName]map[string]int32) map[string]map[string]int32 {
	r := make(map[string]map[string]int32, len(ps))
	for k, v := range ps {
		r[k.String()] = v
	}
	return r
}

func toJSONablePending(pending map[types.NamespacedName]int32) map[string]int32 {
	r := make(map[string]int32, len(pending))
	for k, v := range pending {
		r[k.String()] = v
	}
	return r

}
