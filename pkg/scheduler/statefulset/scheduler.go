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

package statefulset

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/integer"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/scheduler"
	"knative.dev/eventing/pkg/scheduler/factory"
	st "knative.dev/eventing/pkg/scheduler/state"

	_ "knative.dev/eventing/pkg/scheduler/plugins/core/availabilitynodepriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/availabilityzonepriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/evenpodspread"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/lowestordinalpriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/podfitsresources"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/removewithavailabilitynodepriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/removewithavailabilityzonepriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/removewithevenpodspreadpriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/core/removewithhighestordinalpriority"
	_ "knative.dev/eventing/pkg/scheduler/plugins/kafka/nomaxresourcecount"
)

type GetReserved func() map[types.NamespacedName]map[string]int32

type Config struct {
	StatefulSetNamespace string `json:"statefulSetNamespace"`
	StatefulSetName      string `json:"statefulSetName"`

	ScaleCacheConfig scheduler.ScaleCacheConfig `json:"scaleCacheConfig"`
	// PodCapacity max capacity for each StatefulSet's pod.
	PodCapacity int32 `json:"podCapacity"`
	// Autoscaler refresh period
	RefreshPeriod time.Duration `json:"refreshPeriod"`
	// Autoscaler retry period
	RetryPeriod time.Duration `json:"retryPeriod"`

	SchedulerPolicy scheduler.SchedulerPolicyType `json:"schedulerPolicy"`
	SchedPolicy     *scheduler.SchedulerPolicy    `json:"schedPolicy"`
	DeschedPolicy   *scheduler.SchedulerPolicy    `json:"deschedPolicy"`

	Evictor scheduler.Evictor `json:"-"`

	VPodLister scheduler.VPodLister     `json:"-"`
	NodeLister corev1listers.NodeLister `json:"-"`
	// Pod lister for statefulset: StatefulSetNamespace / StatefulSetName
	PodLister corev1listers.PodNamespaceLister `json:"-"`

	// getReserved returns reserved replicas
	getReserved GetReserved
}

func New(ctx context.Context, cfg *Config) (scheduler.Scheduler, error) {

	if cfg.PodLister == nil {
		return nil, fmt.Errorf("Config.PodLister is required")
	}

	scaleCache := scheduler.NewScaleCache(ctx, cfg.StatefulSetNamespace, kubeclient.Get(ctx).AppsV1().StatefulSets(cfg.StatefulSetNamespace), cfg.ScaleCacheConfig)

	stateAccessor := st.NewStateBuilder(cfg.StatefulSetName, cfg.VPodLister, cfg.PodCapacity, cfg.SchedulerPolicy, cfg.SchedPolicy, cfg.DeschedPolicy, cfg.PodLister, cfg.NodeLister, scaleCache)

	var getReserved GetReserved
	cfg.getReserved = func() map[types.NamespacedName]map[string]int32 {
		return getReserved()
	}

	autoscaler := newAutoscaler(cfg, stateAccessor, scaleCache)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Wait()
		autoscaler.Start(ctx)
	}()

	s := newStatefulSetScheduler(ctx, cfg, stateAccessor, autoscaler)
	getReserved = s.Reserved
	wg.Done()

	return s, nil
}

type Pending map[types.NamespacedName]int32

func (p Pending) Total() int32 {
	t := int32(0)
	for _, vr := range p {
		t += vr
	}
	return t
}

// StatefulSetScheduler is a scheduler placing VPod into statefulset-managed set of pods
type StatefulSetScheduler struct {
	statefulSetName      string
	statefulSetNamespace string
	statefulSetClient    clientappsv1.StatefulSetInterface
	vpodLister           scheduler.VPodLister
	lock                 sync.Locker
	stateAccessor        st.StateAccessor
	autoscaler           Autoscaler

	// replicas is the (cached) number of statefulset replicas.
	replicas int32

	// reserved tracks vreplicas that have been placed (ie. scheduled) but haven't been
	// committed yet (ie. not appearing in vpodLister)
	reserved   map[types.NamespacedName]map[string]int32
	reservedMu sync.Mutex
}

var (
	_ reconciler.LeaderAware = &StatefulSetScheduler{}
	_ scheduler.Scheduler    = &StatefulSetScheduler{}
)

// Promote implements reconciler.LeaderAware.
func (s *StatefulSetScheduler) Promote(b reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
	if v, ok := s.autoscaler.(reconciler.LeaderAware); ok {
		return v.Promote(b, enq)
	}
	return nil
}

// Demote implements reconciler.LeaderAware.
func (s *StatefulSetScheduler) Demote(b reconciler.Bucket) {
	if v, ok := s.autoscaler.(reconciler.LeaderAware); ok {
		v.Demote(b)
	}
}

func newStatefulSetScheduler(ctx context.Context,
	cfg *Config,
	stateAccessor st.StateAccessor,
	autoscaler Autoscaler) *StatefulSetScheduler {

	scheduler := &StatefulSetScheduler{
		statefulSetNamespace: cfg.StatefulSetNamespace,
		statefulSetName:      cfg.StatefulSetName,
		statefulSetClient:    kubeclient.Get(ctx).AppsV1().StatefulSets(cfg.StatefulSetNamespace),
		vpodLister:           cfg.VPodLister,
		lock:                 new(sync.Mutex),
		stateAccessor:        stateAccessor,
		reserved:             make(map[types.NamespacedName]map[string]int32),
		autoscaler:           autoscaler,
	}

	// Monitor our statefulset
	c := kubeclient.Get(ctx)
	sif := informers.NewSharedInformerFactoryWithOptions(c,
		controller.GetResyncPeriod(ctx),
		informers.WithNamespace(cfg.StatefulSetNamespace),
	)

	sif.Apps().V1().StatefulSets().Informer().
		AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterWithNameAndNamespace(cfg.StatefulSetNamespace, cfg.StatefulSetName),
			Handler: controller.HandleAll(func(i interface{}) {
				scheduler.updateStatefulset(ctx, i)
			}),
		})

	sif.Start(ctx.Done())
	_ = sif.WaitForCacheSync(ctx.Done())

	go func() {
		<-ctx.Done()
		sif.Shutdown()
	}()

	return scheduler
}

func (s *StatefulSetScheduler) Schedule(ctx context.Context, vpod scheduler.VPod) ([]duckv1alpha1.Placement, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.reservedMu.Lock()
	defer s.reservedMu.Unlock()

	placements, err := s.scheduleVPod(ctx, vpod)
	if placements == nil {
		return placements, err
	}

	sort.SliceStable(placements, func(i int, j int) bool {
		return st.OrdinalFromPodName(placements[i].PodName) < st.OrdinalFromPodName(placements[j].PodName)
	})

	// Reserve new placements until they are committed to the vpod.
	s.reservePlacements(vpod, placements)

	return placements, err
}

func (s *StatefulSetScheduler) scheduleVPod(ctx context.Context, vpod scheduler.VPod) ([]duckv1alpha1.Placement, error) {
	logger := logging.FromContext(ctx).With("key", vpod.GetKey(), zap.String("component", "scheduler"))
	ctx = logging.WithLogger(ctx, logger)

	// Get the current placements state
	// Quite an expensive operation but safe and simple.
	state, err := s.stateAccessor.State(ctx, s.reserved)
	if err != nil {
		logger.Debug("error while refreshing scheduler state (will retry)", zap.Error(err))
		return nil, err
	}

	// Clean up reserved from removed resources that don't appear in the vpod list anymore and have
	// no pending resources.
	reserved := make(map[types.NamespacedName]map[string]int32)
	for k, v := range s.reserved {
		if pendings, ok := state.Pending[k]; ok {
			if pendings == 0 {
				reserved[k] = map[string]int32{}
			} else {
				reserved[k] = v
			}
		}
	}
	s.reserved = reserved

	logger.Debugw("scheduling", zap.Any("state", state))

	existingPlacements := vpod.GetPlacements()
	var left int32

	// Remove unschedulable or adjust overcommitted pods from placements
	var placements []duckv1alpha1.Placement
	if len(existingPlacements) > 0 {
		placements = make([]duckv1alpha1.Placement, 0, len(existingPlacements))
		for _, p := range existingPlacements {
			p := p.DeepCopy()
			ordinal := st.OrdinalFromPodName(p.PodName)

			if !state.IsSchedulablePod(ordinal) {
				continue
			}

			// Handle overcommitted pods.
			if state.Free(ordinal) < 0 {
				// vr > free => vr: 9, overcommit 4 -> free: 0, vr: 5, pending: +4
				// vr = free => vr: 4, overcommit 4 -> free: 0, vr: 0, pending: +4
				// vr < free => vr: 3, overcommit 4 -> free: -1, vr: 0, pending: +3

				overcommit := -state.FreeCap[ordinal]

				logger.Debugw("overcommit", zap.Any("overcommit", overcommit), zap.Any("placement", p))

				if p.VReplicas >= overcommit {
					state.SetFree(ordinal, 0)
					state.Pending[vpod.GetKey()] += overcommit

					p.VReplicas = p.VReplicas - overcommit
				} else {
					state.SetFree(ordinal, p.VReplicas-overcommit)
					state.Pending[vpod.GetKey()] += p.VReplicas

					p.VReplicas = 0
				}
			}

			if p.VReplicas > 0 {
				placements = append(placements, *p)
			}
		}
	}

	// The scheduler when policy type is
	// Policy: MAXFILLUP (SchedulerPolicyType == MAXFILLUP)
	// - allocates as many vreplicas as possible to the same pod(s)
	// - allocates remaining vreplicas to new pods

	// Exact number of vreplicas => do nothing
	tr := scheduler.GetTotalVReplicas(placements)
	if tr == vpod.GetVReplicas() {
		logger.Debug("scheduling succeeded (already scheduled)")

		// Fully placed. Nothing to do
		return placements, nil
	}

	if state.SchedulerPolicy != "" {
		// Need less => scale down
		if tr > vpod.GetVReplicas() {
			logger.Debugw("scaling down", zap.Int32("vreplicas", tr), zap.Int32("new vreplicas", vpod.GetVReplicas()),
				zap.Any("placements", placements),
				zap.Any("existingPlacements", existingPlacements))

			placements = s.removeReplicas(tr-vpod.GetVReplicas(), placements)

			// Do not trigger the autoscaler to avoid unnecessary churn

			return placements, nil
		}

		// Need more => scale up
		logger.Debugw("scaling up", zap.Int32("vreplicas", tr), zap.Int32("new vreplicas", vpod.GetVReplicas()),
			zap.Any("placements", placements),
			zap.Any("existingPlacements", existingPlacements))

		placements, left = s.addReplicas(state, vpod.GetVReplicas()-tr, placements)

	} else { //Predicates and priorities must be used for scheduling
		// Need less => scale down
		if tr > vpod.GetVReplicas() && state.DeschedPolicy != nil {
			logger.Infow("scaling down", zap.Int32("vreplicas", tr), zap.Int32("new vreplicas", vpod.GetVReplicas()),
				zap.Any("placements", placements),
				zap.Any("existingPlacements", existingPlacements))
			placements = s.removeReplicasWithPolicy(ctx, vpod, tr-vpod.GetVReplicas(), placements)

			// Do not trigger the autoscaler to avoid unnecessary churn

			return placements, nil
		}

		if state.SchedPolicy != nil {

			// Need more => scale up
			// rebalancing needed for all vreps most likely since there are pending vreps from previous reconciliation
			// can fall here when vreps scaled up or after eviction
			logger.Infow("scaling up with a rebalance (if needed)", zap.Int32("vreplicas", tr), zap.Int32("new vreplicas", vpod.GetVReplicas()),
				zap.Any("placements", placements),
				zap.Any("existingPlacements", existingPlacements))
			placements, left = s.rebalanceReplicasWithPolicy(ctx, vpod, vpod.GetVReplicas(), placements)
		}
	}

	if left > 0 {
		// Give time for the autoscaler to do its job
		logger.Infow("not enough pod replicas to schedule")

		// Trigger the autoscaler
		if s.autoscaler != nil {
			logger.Infow("Awaiting autoscaler", zap.Any("placement", placements), zap.Int32("left", left))
			s.autoscaler.Autoscale(ctx)
		}

		if state.SchedulerPolicy == "" && state.SchedPolicy != nil {
			logger.Info("reverting to previous placements")
			s.reservePlacements(vpod, existingPlacements)           // rebalancing doesn't care about new placements since all vreps will be re-placed
			return existingPlacements, s.notEnoughPodReplicas(left) // requeue to wait for the autoscaler to do its job
		}

		return placements, s.notEnoughPodReplicas(left)
	}

	logger.Infow("scheduling successful", zap.Any("placement", placements))

	return placements, nil
}

func toJSONable(pending map[types.NamespacedName]int32) map[string]int32 {
	r := make(map[string]int32, len(pending))
	for k, v := range pending {
		r[k.String()] = v
	}
	return r
}

func (s *StatefulSetScheduler) rebalanceReplicasWithPolicy(ctx context.Context, vpod scheduler.VPod, diff int32, placements []duckv1alpha1.Placement) ([]duckv1alpha1.Placement, int32) {
	s.makeZeroPlacements(vpod, placements)
	placements, diff = s.addReplicasWithPolicy(ctx, vpod, diff, make([]duckv1alpha1.Placement, 0)) //start fresh with a new placements list

	return placements, diff
}

func (s *StatefulSetScheduler) removeReplicasWithPolicy(ctx context.Context, vpod scheduler.VPod, diff int32, placements []duckv1alpha1.Placement) []duckv1alpha1.Placement {
	logger := logging.FromContext(ctx).Named("remove replicas with policy")
	numVreps := diff

	for i := int32(0); i < numVreps; i++ { //deschedule one vreplica at a time
		state, err := s.stateAccessor.State(ctx, s.reserved)
		if err != nil {
			logger.Info("error while refreshing scheduler state (will retry)", zap.Error(err))
			return placements
		}

		feasiblePods := s.findFeasiblePods(ctx, state, vpod, state.DeschedPolicy)
		feasiblePods = s.removePodsNotInPlacement(vpod, feasiblePods)
		if len(feasiblePods) == 1 { //nothing to score, remove vrep from that pod
			placementPodID := feasiblePods[0]
			logger.Infof("Selected pod #%v to remove vreplica #%v from", placementPodID, i)
			placements = s.removeSelectionFromPlacements(placementPodID, placements)
			state.SetFree(placementPodID, state.Free(placementPodID)+1)
			s.reservePlacements(vpod, placements)
			continue
		}

		priorityList, err := s.prioritizePods(ctx, state, vpod, feasiblePods, state.DeschedPolicy)
		if err != nil {
			logger.Info("error while scoring pods using priorities", zap.Error(err))
			s.reservePlacements(vpod, placements)
			break
		}

		placementPodID, err := s.selectPod(priorityList)
		if err != nil {
			logger.Info("error while selecting the placement pod", zap.Error(err))
			s.reservePlacements(vpod, placements)
			break
		}

		logger.Infof("Selected pod #%v to remove vreplica #%v from", placementPodID, i)
		placements = s.removeSelectionFromPlacements(placementPodID, placements)
		state.SetFree(placementPodID, state.Free(placementPodID)+1)
		s.reservePlacements(vpod, placements)
	}
	return placements
}

func (s *StatefulSetScheduler) removeSelectionFromPlacements(placementPodID int32, placements []duckv1alpha1.Placement) []duckv1alpha1.Placement {
	newPlacements := make([]duckv1alpha1.Placement, 0, len(placements))

	for i := 0; i < len(placements); i++ {
		ordinal := st.OrdinalFromPodName(placements[i].PodName)
		if placementPodID == ordinal {
			if placements[i].VReplicas == 1 {
				// remove the entire placement
			} else {
				newPlacements = append(newPlacements, duckv1alpha1.Placement{
					PodName:   placements[i].PodName,
					VReplicas: placements[i].VReplicas - 1,
				})
			}
		} else {
			newPlacements = append(newPlacements, duckv1alpha1.Placement{
				PodName:   placements[i].PodName,
				VReplicas: placements[i].VReplicas,
			})
		}
	}
	return newPlacements
}

func (s *StatefulSetScheduler) addReplicasWithPolicy(ctx context.Context, vpod scheduler.VPod, diff int32, placements []duckv1alpha1.Placement) ([]duckv1alpha1.Placement, int32) {
	logger := logging.FromContext(ctx).Named("add replicas with policy")

	numVreps := diff
	for i := int32(0); i < numVreps; i++ { //schedule one vreplica at a time (find most suitable pod placement satisying predicates with high score)
		// Get the current placements state
		state, err := s.stateAccessor.State(ctx, s.reserved)
		if err != nil {
			logger.Info("error while refreshing scheduler state (will retry)", zap.Error(err))
			return placements, diff
		}

		if s.replicas == 0 { //no pods to filter
			logger.Infow("no pods available in statefulset")
			s.reservePlacements(vpod, placements)
			diff = numVreps - i //for autoscaling up
			break               //end the iteration for all vreps since there are not pods
		}

		feasiblePods := s.findFeasiblePods(ctx, state, vpod, state.SchedPolicy)
		if len(feasiblePods) == 0 { //no pods available to schedule this vreplica
			logger.Info("no feasible pods available to schedule this vreplica")
			s.reservePlacements(vpod, placements)
			diff = numVreps - i //for autoscaling up and possible rebalancing
			break
		}

		/* 	if len(feasiblePods) == 1 { //nothing to score, place vrep on that pod (Update: for HA, must run HA scorers)
			placementPodID := feasiblePods[0]
			logger.Infof("Selected pod #%v for vreplica #%v ", placementPodID, i)
			placements = s.addSelectionToPlacements(placementPodID, placements)
			//state.SetFree(placementPodID, state.Free(placementPodID)-1)
			s.reservePlacements(vpod, placements)
			diff--
			continue
		} */

		priorityList, err := s.prioritizePods(ctx, state, vpod, feasiblePods, state.SchedPolicy)
		if err != nil {
			logger.Info("error while scoring pods using priorities", zap.Error(err))
			s.reservePlacements(vpod, placements)
			diff = numVreps - i //for autoscaling up and possible rebalancing
			break
		}

		placementPodID, err := s.selectPod(priorityList)
		if err != nil {
			logger.Info("error while selecting the placement pod", zap.Error(err))
			s.reservePlacements(vpod, placements)
			diff = numVreps - i //for autoscaling up and possible rebalancing
			break
		}

		logger.Infof("Selected pod #%v for vreplica #%v", placementPodID, i)
		placements = s.addSelectionToPlacements(placementPodID, placements)
		state.SetFree(placementPodID, state.Free(placementPodID)-1)
		s.reservePlacements(vpod, placements)
		diff--
	}
	return placements, diff
}

func (s *StatefulSetScheduler) addSelectionToPlacements(placementPodID int32, placements []duckv1alpha1.Placement) []duckv1alpha1.Placement {
	seen := false

	for i := 0; i < len(placements); i++ {
		ordinal := st.OrdinalFromPodName(placements[i].PodName)
		if placementPodID == ordinal {
			seen = true
			placements[i].VReplicas = placements[i].VReplicas + 1
		}
	}
	if !seen {
		placements = append(placements, duckv1alpha1.Placement{
			PodName:   st.PodNameFromOrdinal(s.statefulSetName, placementPodID),
			VReplicas: 1,
		})
	}
	return placements
}

// findFeasiblePods finds the pods that fit the filter plugins
func (s *StatefulSetScheduler) findFeasiblePods(ctx context.Context, state *st.State, vpod scheduler.VPod, policy *scheduler.SchedulerPolicy) []int32 {
	feasiblePods := make([]int32, 0)
	for _, podId := range state.SchedulablePods {
		statusMap := s.RunFilterPlugins(ctx, state, vpod, podId, policy)
		status := statusMap.Merge()
		if status.IsSuccess() {
			feasiblePods = append(feasiblePods, podId)
		}
	}

	return feasiblePods
}

// removePodsNotInPlacement removes pods that do not have vreplicas placed
func (s *StatefulSetScheduler) removePodsNotInPlacement(vpod scheduler.VPod, feasiblePods []int32) []int32 {
	newFeasiblePods := make([]int32, 0)
	for _, e := range vpod.GetPlacements() {
		for _, podID := range feasiblePods {
			if podID == st.OrdinalFromPodName(e.PodName) { //if pod is in current placement list
				newFeasiblePods = append(newFeasiblePods, podID)
			}
		}
	}

	return newFeasiblePods
}

// prioritizePods prioritizes the pods by running the score plugins, which return a score for each pod.
// The scores from each plugin are added together to make the score for that pod.
func (s *StatefulSetScheduler) prioritizePods(ctx context.Context, states *st.State, vpod scheduler.VPod, feasiblePods []int32, policy *scheduler.SchedulerPolicy) (st.PodScoreList, error) {
	logger := logging.FromContext(ctx).Named("prioritize all feasible pods")

	// If no priority configs are provided, then all pods will have a score of one
	result := make(st.PodScoreList, 0, len(feasiblePods))
	if !s.HasScorePlugins(states, policy) {
		for _, podID := range feasiblePods {
			result = append(result, st.PodScore{
				ID:    podID,
				Score: 1,
			})
		}
		return result, nil
	}

	scoresMap, scoreStatus := s.RunScorePlugins(ctx, states, vpod, feasiblePods, policy)
	if !scoreStatus.IsSuccess() {
		logger.Infof("FAILURE! Cannot score feasible pods due to plugin errors %v", scoreStatus.AsError())
		return nil, scoreStatus.AsError()
	}

	// Summarize all scores.
	for i := range feasiblePods {
		result = append(result, st.PodScore{ID: feasiblePods[i], Score: 0})
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}

	return result, nil
}

// selectPod takes a prioritized list of pods and then picks one
func (s *StatefulSetScheduler) selectPod(podScoreList st.PodScoreList) (int32, error) {
	if len(podScoreList) == 0 {
		return -1, fmt.Errorf("empty priority list") //no selected pod
	}

	maxScore := podScoreList[0].Score
	selected := podScoreList[0].ID
	cntOfMaxScore := int64(1)
	for _, ps := range podScoreList[1:] {
		if ps.Score > maxScore {
			maxScore = ps.Score
			selected = ps.ID
			cntOfMaxScore = 1
		} else if ps.Score == maxScore { //if equal scores, randomly picks one
			cntOfMaxScore++
			randNum, err := rand.Int(rand.Reader, big.NewInt(cntOfMaxScore))
			if err != nil {
				return -1, fmt.Errorf("failed to generate random number")
			}
			if randNum.Int64() == int64(0) {
				selected = ps.ID
			}
		}
	}
	return selected, nil
}

// RunFilterPlugins runs the set of configured Filter plugins for a vrep on the given pod.
// If any of these plugins doesn't return "Success", the pod is not suitable for placing the vrep.
// Meanwhile, the failure message and status are set for the given pod.
func (s *StatefulSetScheduler) RunFilterPlugins(ctx context.Context, states *st.State, vpod scheduler.VPod, podID int32, policy *scheduler.SchedulerPolicy) st.PluginToStatus {
	logger := logging.FromContext(ctx).Named("run all filter plugins")

	statuses := make(st.PluginToStatus)
	for _, plugin := range policy.Predicates {
		pl, err := factory.GetFilterPlugin(plugin.Name)
		if err != nil {
			logger.Error("Could not find filter plugin in Registry: ", plugin.Name)
			continue
		}

		//logger.Infof("Going to run filter plugin: %s using state: %v ", pl.Name(), states)
		pluginStatus := s.runFilterPlugin(ctx, pl, plugin.Args, states, vpod, podID)
		if !pluginStatus.IsSuccess() {
			if !pluginStatus.IsUnschedulable() {
				errStatus := st.NewStatus(st.Error, fmt.Sprintf("running %q filter plugin for pod %q failed with: %v", pl.Name(), podID, pluginStatus.Message()))
				return map[string]*st.Status{pl.Name(): errStatus} //TODO: if one plugin fails, then no more plugins are run
			}
			statuses[pl.Name()] = pluginStatus
			return statuses
		}
	}

	return statuses
}

func (s *StatefulSetScheduler) runFilterPlugin(ctx context.Context, pl st.FilterPlugin, args interface{}, states *st.State, vpod scheduler.VPod, podID int32) *st.Status {
	status := pl.Filter(ctx, args, states, vpod.GetKey(), podID)
	return status
}

// RunScorePlugins runs the set of configured scoring plugins. It returns a list that stores for each scoring plugin name the corresponding PodScoreList(s).
// It also returns *Status, which is set to non-success if any of the plugins returns a non-success status.
func (s *StatefulSetScheduler) RunScorePlugins(ctx context.Context, states *st.State, vpod scheduler.VPod, feasiblePods []int32, policy *scheduler.SchedulerPolicy) (st.PluginToPodScores, *st.Status) {
	logger := logging.FromContext(ctx).Named("run all score plugins")

	pluginToPodScores := make(st.PluginToPodScores, len(policy.Priorities))
	for _, plugin := range policy.Priorities {
		pl, err := factory.GetScorePlugin(plugin.Name)
		if err != nil {
			logger.Error("Could not find score plugin in registry: ", plugin.Name)
			continue
		}

		//logger.Infof("Going to run score plugin: %s using state: %v ", pl.Name(), states)
		pluginToPodScores[pl.Name()] = make(st.PodScoreList, len(feasiblePods))
		for index, podID := range feasiblePods {
			score, pluginStatus := s.runScorePlugin(ctx, pl, plugin.Args, states, feasiblePods, vpod, podID)
			if !pluginStatus.IsSuccess() {
				errStatus := st.NewStatus(st.Error, fmt.Sprintf("running %q scoring plugin for pod %q failed with: %v", pl.Name(), podID, pluginStatus.AsError()))
				return pluginToPodScores, errStatus //TODO: if one plugin fails, then no more plugins are run
			}

			score = score * plugin.Weight //WEIGHED SCORE VALUE
			//logger.Infof("scoring plugin %q produced score %v for pod %q: %v", pl.Name(), score, podID, pluginStatus)
			pluginToPodScores[pl.Name()][index] = st.PodScore{
				ID:    podID,
				Score: score,
			}
		}

		status := pl.ScoreExtensions().NormalizeScore(ctx, states, pluginToPodScores[pl.Name()]) //NORMALIZE SCORES FOR ALL FEASIBLE PODS
		if !status.IsSuccess() {
			errStatus := st.NewStatus(st.Error, fmt.Sprintf("running %q scoring plugin failed with: %v", pl.Name(), status.AsError()))
			return pluginToPodScores, errStatus
		}
	}

	return pluginToPodScores, st.NewStatus(st.Success)
}

func (s *StatefulSetScheduler) runScorePlugin(ctx context.Context, pl st.ScorePlugin, args interface{}, states *st.State, feasiblePods []int32, vpod scheduler.VPod, podID int32) (uint64, *st.Status) {
	score, status := pl.Score(ctx, args, states, feasiblePods, vpod.GetKey(), podID)
	return score, status
}

// HasScorePlugins returns true if at least one score plugin is defined.
func (s *StatefulSetScheduler) HasScorePlugins(state *st.State, policy *scheduler.SchedulerPolicy) bool {
	return len(policy.Priorities) > 0
}

func (s *StatefulSetScheduler) removeReplicas(diff int32, placements []duckv1alpha1.Placement) []duckv1alpha1.Placement {
	newPlacements := make([]duckv1alpha1.Placement, 0, len(placements))
	for i := len(placements) - 1; i > -1; i-- {
		if diff >= placements[i].VReplicas {
			// remove the entire placement
			diff -= placements[i].VReplicas
		} else {
			newPlacements = append(newPlacements, duckv1alpha1.Placement{
				PodName:   placements[i].PodName,
				VReplicas: placements[i].VReplicas - diff,
			})
			diff = 0
		}
	}
	return newPlacements
}

func (s *StatefulSetScheduler) addReplicas(states *st.State, diff int32, placements []duckv1alpha1.Placement) ([]duckv1alpha1.Placement, int32) {
	// Pod affinity algorithm: prefer adding replicas to existing pods before considering other replicas
	newPlacements := make([]duckv1alpha1.Placement, 0, len(placements))

	// Add to existing
	for i := 0; i < len(placements); i++ {
		podName := placements[i].PodName
		ordinal := st.OrdinalFromPodName(podName)

		// Is there space in PodName?
		f := states.Free(ordinal)
		if diff >= 0 && f > 0 {
			allocation := integer.Int32Min(f, diff)
			newPlacements = append(newPlacements, duckv1alpha1.Placement{
				PodName:   podName,
				VReplicas: placements[i].VReplicas + allocation,
			})

			diff -= allocation
			states.SetFree(ordinal, f-allocation)
		} else {
			newPlacements = append(newPlacements, placements[i])
		}
	}

	if diff > 0 {
		// Needs to allocate replicas to additional pods
		for ordinal := int32(0); ordinal < s.replicas; ordinal++ {
			f := states.Free(ordinal)
			if f > 0 {
				allocation := integer.Int32Min(f, diff)
				newPlacements = append(newPlacements, duckv1alpha1.Placement{
					PodName:   st.PodNameFromOrdinal(s.statefulSetName, ordinal),
					VReplicas: allocation,
				})

				diff -= allocation
				states.SetFree(ordinal, f-allocation)
			}

			if diff == 0 {
				break
			}
		}
	}

	return newPlacements, diff
}

func (s *StatefulSetScheduler) updateStatefulset(ctx context.Context, obj interface{}) {
	statefulset, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		logging.FromContext(ctx).Warnw("expected a Statefulset object", zap.Any("object", obj))
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if statefulset.Spec.Replicas == nil {
		s.replicas = 1
	} else if s.replicas != *statefulset.Spec.Replicas {
		s.replicas = *statefulset.Spec.Replicas
		logging.FromContext(ctx).Infow("statefulset replicas updated", zap.Int32("replicas", s.replicas))
	}
}

func (s *StatefulSetScheduler) reservePlacements(vpod scheduler.VPod, placements []duckv1alpha1.Placement) {
	if len(placements) == 0 { // clear our old placements in reserved
		s.reserved[vpod.GetKey()] = make(map[string]int32)
	}

	for _, p := range placements {
		// note: track all vreplicas, not only the new ones since
		// the next time `state()` is called some vreplicas might
		// have been committed.
		if _, ok := s.reserved[vpod.GetKey()]; !ok {
			s.reserved[vpod.GetKey()] = make(map[string]int32)
		}
		s.reserved[vpod.GetKey()][p.PodName] = p.VReplicas
	}
}

func (s *StatefulSetScheduler) makeZeroPlacements(vpod scheduler.VPod, placements []duckv1alpha1.Placement) {
	newPlacements := make([]duckv1alpha1.Placement, len(placements))
	for i := 0; i < len(placements); i++ {
		newPlacements[i].PodName = placements[i].PodName
		newPlacements[i].VReplicas = 0
	}
	// This is necessary to make sure State() zeroes out initial pod/node/zone spread and
	// free capacity when there are existing placements for a vpod
	s.reservePlacements(vpod, newPlacements)
}

// newNotEnoughPodReplicas returns an error explaining what is the problem, what are the actions we're taking
// to try to fix it (retry), wrapping a controller.requeueKeyError which signals to ReconcileKind to requeue the
// object after a given delay.
func (s *StatefulSetScheduler) notEnoughPodReplicas(left int32) error {
	// Wrap controller.requeueKeyError error to wait for the autoscaler to do its job.
	return fmt.Errorf("insufficient running pods replicas for StatefulSet %s/%s to schedule resource replicas (left: %d): retry %w",
		s.statefulSetNamespace, s.statefulSetName,
		left,
		controller.NewRequeueAfter(5*time.Second),
	)
}

func (s *StatefulSetScheduler) Reserved() map[types.NamespacedName]map[string]int32 {
	s.reservedMu.Lock()
	defer s.reservedMu.Unlock()

	r := make(map[types.NamespacedName]map[string]int32, len(s.reserved))
	for k1, v1 := range s.reserved {
		r[k1] = make(map[string]int32, len(v1))
		for k2, v2 := range v1 {
			r[k1][k2] = v2
		}
	}

	return r
}
