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
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/scheduler"
	st "knative.dev/eventing/pkg/scheduler/state"
)

type GetReserved func() map[types.NamespacedName]map[string]int32

type Config struct {
	StatefulSetNamespace string `json:"statefulSetNamespace"`
	StatefulSetName      string `json:"statefulSetName"`

	ScaleCacheConfig scheduler.ScaleCacheConfig `json:"scaleCacheConfig"`
	// PodCapacity max capacity for each StatefulSet's pod.
	PodCapacity int32 `json:"podCapacity"`
	// MinReplicas is the minimum replicas of the statefulset.
	MinReplicas int32 `json:"minReplicas"`
	// Autoscaler refresh period
	RefreshPeriod time.Duration `json:"refreshPeriod"`
	// Autoscaler retry period
	RetryPeriod time.Duration `json:"retryPeriod"`

	Evictor scheduler.Evictor `json:"-"`

	VPodLister scheduler.VPodLister `json:"-"`
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

	stateAccessor := st.NewStateBuilder(cfg.StatefulSetName, cfg.VPodLister, cfg.PodCapacity, cfg.PodLister, scaleCache)

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

	// isLeader signals whether a given Scheduler instance is leader or not.
	// The autoscaler is considered the leader when ephemeralLeaderElectionObject is in a
	// bucket where we've been promoted.
	isLeader atomic.Bool

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
	if !b.Has(ephemeralLeaderElectionObject) {
		return nil
	}
	// The demoted bucket has the ephemeralLeaderElectionObject, so we are not leader anymore.
	// Flip the flag after running initReserved.
	defer s.isLeader.Store(true)

	if v, ok := s.autoscaler.(reconciler.LeaderAware); ok {
		return v.Promote(b, enq)
	}
	if err := s.initReserved(); err != nil {
		return err
	}
	return nil
}

func (s *StatefulSetScheduler) initReserved() error {
	s.reservedMu.Lock()
	defer s.reservedMu.Unlock()

	vPods, err := s.vpodLister()
	if err != nil {
		return fmt.Errorf("failed to list vPods during init: %w", err)
	}

	s.reserved = make(map[types.NamespacedName]map[string]int32, len(vPods))
	for _, vPod := range vPods {
		if !vPod.GetDeletionTimestamp().IsZero() {
			continue
		}
		s.reserved[vPod.GetKey()] = make(map[string]int32, len(vPod.GetPlacements()))
		for _, placement := range vPod.GetPlacements() {
			s.reserved[vPod.GetKey()][placement.PodName] += placement.VReplicas
		}
	}
	return nil
}

// resyncReserved removes deleted vPods from reserved to keep the state consistent when leadership
// changes (Promote / Demote).
// initReserved is not enough since the vPod lister can be stale.
func (s *StatefulSetScheduler) resyncReserved() error {
	if !s.isLeader.Load() {
		return nil
	}

	vPods, err := s.vpodLister()
	if err != nil {
		return fmt.Errorf("failed to list vPods during reserved resync: %w", err)
	}
	vPodsByK := vPodsByKey(vPods)

	s.reservedMu.Lock()
	defer s.reservedMu.Unlock()

	for key := range s.reserved {
		vPod, ok := vPodsByK[key]
		if !ok || vPod == nil {
			delete(s.reserved, key)
		}
	}

	return nil
}

// Demote implements reconciler.LeaderAware.
func (s *StatefulSetScheduler) Demote(b reconciler.Bucket) {
	if !b.Has(ephemeralLeaderElectionObject) {
		return
	}
	// The demoted bucket has the ephemeralLeaderElectionObject, so we are not leader anymore.
	defer s.isLeader.Store(false)

	if v, ok := s.autoscaler.(reconciler.LeaderAware); ok {
		v.Demote(b)
	}
}

func newStatefulSetScheduler(ctx context.Context,
	cfg *Config,
	stateAccessor st.StateAccessor,
	autoscaler Autoscaler) *StatefulSetScheduler {

	s := &StatefulSetScheduler{
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

	_, err := sif.Apps().V1().StatefulSets().Informer().
		AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterWithNameAndNamespace(cfg.StatefulSetNamespace, cfg.StatefulSetName),
			Handler: controller.HandleAll(func(i interface{}) {
				s.updateStatefulset(ctx, i)
			}),
		})
	if err != nil {
		logging.FromContext(ctx).Fatalw("Failed to register informer", zap.Error(err))
	}

	sif.Start(ctx.Done())
	_ = sif.WaitForCacheSync(ctx.Done())

	go func() {
		<-ctx.Done()
		sif.Shutdown()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.RefreshPeriod * 3):
				_ = s.resyncReserved()
			}
		}
	}()

	return s
}

func (s *StatefulSetScheduler) Schedule(ctx context.Context, vpod scheduler.VPod) ([]duckv1alpha1.Placement, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.reservedMu.Lock()
	defer s.reservedMu.Unlock()

	placements, err := s.scheduleVPod(ctx, vpod)

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
	state, err := s.stateAccessor.State(ctx)
	if err != nil {
		logger.Debug("error while refreshing scheduler state (will retry)", zap.Error(err))
		return nil, err
	}

	reservedByPodName := make(map[string]int32, 2)
	for _, v := range s.reserved {
		for podName, vReplicas := range v {
			v, _ := reservedByPodName[podName]
			reservedByPodName[podName] = vReplicas + v
		}
	}

	// Use reserved placements as starting point, if we have them.
	existingPlacements := make([]duckv1alpha1.Placement, 0)
	if placements, ok := s.reserved[vpod.GetKey()]; ok {
		existingPlacements = make([]duckv1alpha1.Placement, 0, len(placements))
		for podName, n := range placements {
			existingPlacements = append(existingPlacements, duckv1alpha1.Placement{
				PodName:   podName,
				VReplicas: n,
			})
		}
	}

	sort.SliceStable(existingPlacements, func(i int, j int) bool {
		return st.OrdinalFromPodName(existingPlacements[i].PodName) < st.OrdinalFromPodName(existingPlacements[j].PodName)
	})

	logger.Debugw("scheduling state",
		zap.Any("state", state),
		zap.Any("reservedByPodName", reservedByPodName),
		zap.Any("reserved", st.ToJSONable(s.reserved)),
		zap.Any("vpod", vpod),
	)

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
			reserved, _ := reservedByPodName[p.PodName]
			if state.Capacity-reserved < 0 {
				// vr > free => vr: 9, overcommit 4 -> free: 0, vr: 5, pending: +4
				// vr = free => vr: 4, overcommit 4 -> free: 0, vr: 0, pending: +4
				// vr < free => vr: 3, overcommit 4 -> free: -1, vr: 0, pending: +3

				overcommit := -(state.Capacity - reserved)

				logger.Debugw("overcommit", zap.Any("overcommit", overcommit), zap.Any("placement", p))

				if p.VReplicas >= overcommit {
					state.SetFree(ordinal, 0)
					state.Pending[vpod.GetKey()] += overcommit
					reservedByPodName[p.PodName] -= overcommit

					p.VReplicas = p.VReplicas - overcommit
				} else {
					state.SetFree(ordinal, p.VReplicas-overcommit)
					state.Pending[vpod.GetKey()] += p.VReplicas
					reservedByPodName[p.PodName] -= p.VReplicas

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

	placements, left := s.addReplicas(state, reservedByPodName, vpod, vpod.GetVReplicas()-tr, placements)

	if left > 0 {
		// Give time for the autoscaler to do its job
		logger.Infow("not enough pod replicas to schedule")

		// Trigger the autoscaler
		if s.autoscaler != nil {
			logger.Infow("Awaiting autoscaler", zap.Any("placement", placements), zap.Int32("left", left))
			s.autoscaler.Autoscale(ctx)
		}

		return placements, s.notEnoughPodReplicas(left)
	}

	logger.Infow("scheduling successful", zap.Any("placement", placements))

	return placements, nil
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

func (s *StatefulSetScheduler) addReplicas(states *st.State, reservedByPodName map[string]int32, vpod scheduler.VPod, diff int32, placements []duckv1alpha1.Placement) ([]duckv1alpha1.Placement, int32) {
	if states.Replicas <= 0 {
		return placements, diff
	}

	newPlacements := make([]duckv1alpha1.Placement, 0, len(placements))

	// Preserve existing placements
	for _, p := range placements {
		newPlacements = append(newPlacements, *p.DeepCopy())
	}

	candidates := s.candidatesOrdered(states, vpod, placements)

	// Spread replicas in as many candidates as possible.
	foundFreeCandidate := true
	for diff > 0 && foundFreeCandidate {
		foundFreeCandidate = false
		for _, ordinal := range candidates {
			if diff <= 0 {
				break
			}

			podName := st.PodNameFromOrdinal(states.StatefulSetName, ordinal)
			reserved, _ := reservedByPodName[podName]
			// Is there space?
			if states.Capacity-reserved > 0 {
				foundFreeCandidate = true
				allocation := int32(1)

				newPlacements = upsertPlacements(newPlacements, duckv1alpha1.Placement{
					PodName:   st.PodNameFromOrdinal(states.StatefulSetName, ordinal),
					VReplicas: allocation,
				})

				diff -= allocation
				reservedByPodName[podName] += allocation
			}
		}
	}

	if len(newPlacements) == 0 {
		return nil, diff
	}
	return newPlacements, diff
}

func (s *StatefulSetScheduler) candidatesOrdered(states *st.State, vpod scheduler.VPod, placements []duckv1alpha1.Placement) []int32 {
	existingPlacements := sets.New[string]()
	candidates := make([]int32, len(states.SchedulablePods))

	firstIdx := 0
	lastIdx := len(candidates) - 1

	// De-prioritize existing placements pods, add existing placements to the tail of the candidates.
	// Start from the last one so that within the "existing replicas" group, we prioritize lower ordinals
	// to reduce compaction.
	for i := len(placements) - 1; i >= 0; i-- {
		placement := placements[i]
		ordinal := st.OrdinalFromPodName(placement.PodName)
		if !states.IsSchedulablePod(ordinal) {
			continue
		}
		// This should really never happen as placements are de-duped, however, better to handle
		// edge cases in case the prerequisite doesn't hold in the future.
		if existingPlacements.Has(placement.PodName) {
			continue
		}
		candidates[lastIdx] = ordinal
		lastIdx--
		existingPlacements.Insert(placement.PodName)
	}

	// Prioritize reserved placements that don't appear in the committed placements.
	if reserved, ok := s.reserved[vpod.GetKey()]; ok {
		for podName := range reserved {
			if !states.IsSchedulablePod(st.OrdinalFromPodName(podName)) {
				continue
			}
			if existingPlacements.Has(podName) {
				continue
			}
			candidates[firstIdx] = st.OrdinalFromPodName(podName)
			firstIdx++
			existingPlacements.Insert(podName)
		}
	}

	// Add all the ordinals to the candidates list.
	// De-prioritize the last ordinals over lower ordinals so that we reduce the chances for compaction.
	for ordinal := s.replicas - 1; ordinal >= 0; ordinal-- {
		if !states.IsSchedulablePod(ordinal) {
			continue
		}
		podName := st.PodNameFromOrdinal(states.StatefulSetName, ordinal)
		if existingPlacements.Has(podName) {
			continue
		}
		candidates[lastIdx] = ordinal
		lastIdx--
	}
	return candidates
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
		delete(s.reserved, vpod.GetKey())
		return
	}

	s.reserved[vpod.GetKey()] = make(map[string]int32, len(placements))

	for _, p := range placements {
		s.reserved[vpod.GetKey()][p.PodName] = p.VReplicas
	}
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

func upsertPlacements(placements []duckv1alpha1.Placement, placement duckv1alpha1.Placement) []duckv1alpha1.Placement {
	found := false
	for i := range placements {
		if placements[i].PodName == placement.PodName {
			placements[i].VReplicas = placements[i].VReplicas + placement.VReplicas
			found = true
			break
		}
	}
	if !found {
		placements = append(placements, placement)
	}
	return placements
}

func vPodsByKey(vPods []scheduler.VPod) map[types.NamespacedName]scheduler.VPod {
	r := make(map[types.NamespacedName]scheduler.VPod, len(vPods))
	for _, vPod := range vPods {
		r[vPod.GetKey()] = vPod
	}
	return r
}
