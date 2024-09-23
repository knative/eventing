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
	"errors"
	"math"
	"strconv"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	State(ctx context.Context, reserved map[types.NamespacedName]map[string]int32) (*State, error)
}

// state provides information about the current scheduling of all vpods
// It is used by for the scheduler and the autoscaler
type State struct {
	// free tracks the free capacity of each pod.
	FreeCap []int32

	// schedulable pods tracks the pods that aren't being evicted.
	SchedulablePods []int32

	// LastOrdinal is the ordinal index corresponding to the last statefulset replica
	// with placed vpods.
	LastOrdinal int32

	// Pod capacity.
	Capacity int32

	// Replicas is the (cached) number of statefulset replicas.
	Replicas int32

	// Number of available zones in cluster
	NumZones int32

	// Number of available nodes in cluster
	NumNodes int32

	// Scheduling policy type for placing vreplicas on pods
	SchedulerPolicy scheduler.SchedulerPolicyType

	// Scheduling policy plugin for placing vreplicas on pods
	SchedPolicy *scheduler.SchedulerPolicy

	// De-scheduling policy plugin for removing vreplicas from pods
	DeschedPolicy *scheduler.SchedulerPolicy

	// Mapping node names of nodes currently in cluster to their zone info
	NodeToZoneMap map[string]string

	StatefulSetName string

	PodLister corev1.PodNamespaceLister

	// Stores for each vpod, a map of podname to number of vreplicas placed on that pod currently
	PodSpread map[types.NamespacedName]map[string]int32

	// Stores for each vpod, a map of nodename to total number of vreplicas placed on all pods running on that node currently
	NodeSpread map[types.NamespacedName]map[string]int32

	// Stores for each vpod, a map of zonename to total number of vreplicas placed on all pods located in that zone currently
	ZoneSpread map[types.NamespacedName]map[string]int32

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

// freeCapacity returns the number of vreplicas that can be used,
// up to the last ordinal
func (s *State) FreeCapacity() int32 {
	t := int32(0)
	for _, i := range s.SchedulablePods {
		t += s.FreeCap[i]
	}
	return t
}

func (s *State) GetPodInfo(podName string) (zoneName string, nodeName string, err error) {
	pod, err := s.PodLister.Get(podName)
	if err != nil {
		return zoneName, nodeName, err
	}

	nodeName = pod.Spec.NodeName
	zoneName, ok := s.NodeToZoneMap[nodeName]
	if !ok {
		return zoneName, nodeName, errors.New("could not find zone")
	}
	return zoneName, nodeName, nil
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
	schedulerPolicy  scheduler.SchedulerPolicyType
	nodeLister       corev1.NodeLister
	statefulSetCache *scheduler.ScaleCache
	statefulSetName  string
	podLister        corev1.PodNamespaceLister
	schedPolicy      *scheduler.SchedulerPolicy
	deschedPolicy    *scheduler.SchedulerPolicy
}

// NewStateBuilder returns a StateAccessor recreating the state from scratch each time it is requested
func NewStateBuilder(sfsname string, lister scheduler.VPodLister, podCapacity int32, schedulerPolicy scheduler.SchedulerPolicyType, schedPolicy, deschedPolicy *scheduler.SchedulerPolicy, podlister corev1.PodNamespaceLister, nodeLister corev1.NodeLister, statefulSetCache *scheduler.ScaleCache) StateAccessor {

	return &stateBuilder{
		vpodLister:       lister,
		capacity:         podCapacity,
		schedulerPolicy:  schedulerPolicy,
		nodeLister:       nodeLister,
		statefulSetCache: statefulSetCache,
		statefulSetName:  sfsname,
		podLister:        podlister,
		schedPolicy:      schedPolicy,
		deschedPolicy:    deschedPolicy,
	}
}

func (s *stateBuilder) State(ctx context.Context, reserved map[types.NamespacedName]map[string]int32) (*State, error) {
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

	free := make([]int32, 0)
	pending := make(map[types.NamespacedName]int32, 4)
	expectedVReplicasByVPod := make(map[types.NamespacedName]int32, len(vpods))
	schedulablePods := sets.NewInt32()
	last := int32(-1)

	// keep track of (vpod key, podname) pairs with existing placements
	withPlacement := make(map[types.NamespacedName]map[string]bool)

	podSpread := make(map[types.NamespacedName]map[string]int32)
	nodeSpread := make(map[types.NamespacedName]map[string]int32)
	zoneSpread := make(map[types.NamespacedName]map[string]int32)

	//Build the node to zone map
	nodes, err := s.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	nodeToZoneMap := make(map[string]string)
	zoneMap := make(map[string]struct{})
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]

		if isNodeUnschedulable(node) {
			// Ignore node that is currently unschedulable.
			continue
		}

		zoneName, ok := node.GetLabels()[scheduler.ZoneLabel]
		if ok && zoneName != "" {
			nodeToZoneMap[node.Name] = zoneName
			zoneMap[zoneName] = struct{}{}
		} else {
			nodeToZoneMap[node.Name] = scheduler.UnknownZone
			zoneMap[scheduler.UnknownZone] = struct{}{}
		}
	}

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

		node, err := s.nodeLister.Get(pod.Spec.NodeName)
		if err != nil {
			return nil, err
		}

		if isNodeUnschedulable(node) {
			// Node is marked as Unschedulable - CANNOT SCHEDULE VREPS on a pod running on this node.
			logger.Debugw("Pod is on an unschedulable node", zap.Any("pod", node))
			continue
		}

		// Pod has no annotation or not annotated as unschedulable and
		// not on an unschedulable node, so add to feasible
		schedulablePods.Insert(podId)
	}

	for _, p := range schedulablePods.List() {
		free, last = s.updateFreeCapacity(logger, free, last, PodNameFromOrdinal(s.statefulSetName, p), 0)
	}

	// Getting current state from existing placements for all vpods
	for _, vpod := range vpods {
		ps := vpod.GetPlacements()

		pending[vpod.GetKey()] = pendingFromVPod(vpod)
		expectedVReplicasByVPod[vpod.GetKey()] = vpod.GetVReplicas()

		withPlacement[vpod.GetKey()] = make(map[string]bool)
		podSpread[vpod.GetKey()] = make(map[string]int32)
		nodeSpread[vpod.GetKey()] = make(map[string]int32)
		zoneSpread[vpod.GetKey()] = make(map[string]int32)

		for i := 0; i < len(ps); i++ {
			podName := ps[i].PodName
			vreplicas := ps[i].VReplicas

			// Account for reserved vreplicas
			vreplicas = withReserved(vpod.GetKey(), podName, vreplicas, reserved)

			free, last = s.updateFreeCapacity(logger, free, last, podName, vreplicas)

			withPlacement[vpod.GetKey()][podName] = true

			pod, err := s.podLister.Get(podName)
			if err != nil {
				logger.Warnw("Failed to get pod", zap.String("podName", podName), zap.Error(err))
			}

			if pod != nil && schedulablePods.Has(OrdinalFromPodName(pod.GetName())) {
				nodeName := pod.Spec.NodeName       //node name for this pod
				zoneName := nodeToZoneMap[nodeName] //zone name for this pod
				podSpread[vpod.GetKey()][podName] = podSpread[vpod.GetKey()][podName] + vreplicas
				nodeSpread[vpod.GetKey()][nodeName] = nodeSpread[vpod.GetKey()][nodeName] + vreplicas
				zoneSpread[vpod.GetKey()][zoneName] = zoneSpread[vpod.GetKey()][zoneName] + vreplicas
			}
		}
	}

	// Account for reserved vreplicas with no prior placements
	for key, ps := range reserved {
		for podName, rvreplicas := range ps {
			if wp, ok := withPlacement[key]; ok {
				if _, ok := wp[podName]; ok {
					// already accounted for
					continue
				}

				pod, err := s.podLister.Get(podName)
				if err != nil {
					logger.Warnw("Failed to get pod", zap.String("podName", podName), zap.Error(err))
				}

				if pod != nil && schedulablePods.Has(OrdinalFromPodName(pod.GetName())) {
					nodeName := pod.Spec.NodeName       //node name for this pod
					zoneName := nodeToZoneMap[nodeName] //zone name for this pod
					podSpread[key][podName] = podSpread[key][podName] + rvreplicas
					nodeSpread[key][nodeName] = nodeSpread[key][nodeName] + rvreplicas
					zoneSpread[key][zoneName] = zoneSpread[key][zoneName] + rvreplicas
				}
			}

			free, last = s.updateFreeCapacity(logger, free, last, podName, rvreplicas)
		}
	}

	state := &State{FreeCap: free, SchedulablePods: schedulablePods.List(), LastOrdinal: last, Capacity: s.capacity, Replicas: scale.Spec.Replicas, NumZones: int32(len(zoneMap)), NumNodes: int32(len(nodeToZoneMap)),
		SchedulerPolicy: s.schedulerPolicy, SchedPolicy: s.schedPolicy, DeschedPolicy: s.deschedPolicy, NodeToZoneMap: nodeToZoneMap, StatefulSetName: s.statefulSetName, PodLister: s.podLister,
		PodSpread: podSpread, NodeSpread: nodeSpread, ZoneSpread: zoneSpread, Pending: pending, ExpectedVReplicaByVPod: expectedVReplicasByVPod}

	logger.Infow("cluster state info", zap.Any("state", state), zap.Any("reserved", toJSONable(reserved)))

	return state, nil
}

func pendingFromVPod(vpod scheduler.VPod) int32 {
	expected := vpod.GetVReplicas()
	scheduled := scheduler.GetTotalVReplicas(vpod.GetPlacements())

	return int32(math.Max(float64(0), float64(expected-scheduled)))
}

func (s *stateBuilder) updateFreeCapacity(logger *zap.SugaredLogger, free []int32, last int32, podName string, vreplicas int32) ([]int32, int32) {
	ordinal := OrdinalFromPodName(podName)
	free = grow(free, ordinal, s.capacity)

	free[ordinal] -= vreplicas

	// Assert the pod is not overcommitted
	if free[ordinal] < 0 {
		// This should not happen anymore. Log as an error but do not interrupt the current scheduling.
		logger.Warnw("pod is overcommitted", zap.String("podName", podName), zap.Int32("free", free[ordinal]))
	}

	if ordinal > last {
		last = ordinal
	}

	return free, last
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

func withReserved(key types.NamespacedName, podName string, committed int32, reserved map[types.NamespacedName]map[string]int32) int32 {
	if reserved != nil {
		if rps, ok := reserved[key]; ok {
			if rvreplicas, ok := rps[podName]; ok {
				if committed == rvreplicas {
					// new placement has been committed.
					delete(rps, podName)
					if len(rps) == 0 {
						delete(reserved, key)
					}
				} else {
					// new placement hasn't been committed yet. Adjust locally
					// needed for descheduling vreps using policies
					return rvreplicas
				}
			}
		}
	}
	return committed
}

func isPodUnschedulable(pod *v1.Pod) bool {
	annotVal, ok := pod.ObjectMeta.Annotations[scheduler.PodAnnotationKey]
	unschedulable, err := strconv.ParseBool(annotVal)

	isMarkedUnschedulable := ok && err == nil && unschedulable
	isPending := pod.Spec.NodeName == ""

	return isMarkedUnschedulable || isPending
}

func isNodeUnschedulable(node *v1.Node) bool {
	noExec := &v1.Taint{
		Key:    "node.kubernetes.io/unreachable",
		Effect: v1.TaintEffectNoExecute,
	}

	noSched := &v1.Taint{
		Key:    "node.kubernetes.io/unreachable",
		Effect: v1.TaintEffectNoSchedule,
	}

	return node.Spec.Unschedulable ||
		contains(node.Spec.Taints, noExec) ||
		contains(node.Spec.Taints, noSched)
}

func contains(taints []v1.Taint, taint *v1.Taint) bool {
	for _, v := range taints {
		if v.MatchTaint(taint) {
			return true
		}
	}
	return false
}

func (s *State) MarshalJSON() ([]byte, error) {

	type S struct {
		FreeCap         []int32                       `json:"freeCap"`
		SchedulablePods []int32                       `json:"schedulablePods"`
		LastOrdinal     int32                         `json:"lastOrdinal"`
		Capacity        int32                         `json:"capacity"`
		Replicas        int32                         `json:"replicas"`
		NumZones        int32                         `json:"numZones"`
		NumNodes        int32                         `json:"numNodes"`
		NodeToZoneMap   map[string]string             `json:"nodeToZoneMap"`
		StatefulSetName string                        `json:"statefulSetName"`
		PodSpread       map[string]map[string]int32   `json:"podSpread"`
		NodeSpread      map[string]map[string]int32   `json:"nodeSpread"`
		ZoneSpread      map[string]map[string]int32   `json:"zoneSpread"`
		SchedulerPolicy scheduler.SchedulerPolicyType `json:"schedulerPolicy"`
		SchedPolicy     *scheduler.SchedulerPolicy    `json:"schedPolicy"`
		DeschedPolicy   *scheduler.SchedulerPolicy    `json:"deschedPolicy"`
		Pending         map[string]int32              `json:"pending"`
	}

	sj := S{
		FreeCap:         s.FreeCap,
		SchedulablePods: s.SchedulablePods,
		LastOrdinal:     s.LastOrdinal,
		Capacity:        s.Capacity,
		Replicas:        s.Replicas,
		NumZones:        s.NumZones,
		NumNodes:        s.NumNodes,
		NodeToZoneMap:   s.NodeToZoneMap,
		StatefulSetName: s.StatefulSetName,
		PodSpread:       toJSONable(s.PodSpread),
		NodeSpread:      toJSONable(s.NodeSpread),
		ZoneSpread:      toJSONable(s.ZoneSpread),
		SchedulerPolicy: s.SchedulerPolicy,
		SchedPolicy:     s.SchedPolicy,
		DeschedPolicy:   s.DeschedPolicy,
		Pending:         toJSONablePending(s.Pending),
	}

	return json.Marshal(sj)
}

func toJSONable(ps map[types.NamespacedName]map[string]int32) map[string]map[string]int32 {
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
