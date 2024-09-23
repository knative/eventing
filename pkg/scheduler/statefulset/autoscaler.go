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
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing/pkg/scheduler"
	st "knative.dev/eventing/pkg/scheduler/state"
)

var (
	// ephemeralLeaderElectionObject is the key used to check whether a given autoscaler instance
	// is leader or not.
	// This is an ephemeral key and must be kept stable and unmodified across releases.
	ephemeralLeaderElectionObject = types.NamespacedName{
		Namespace: "knative-eventing",
		Name:      "autoscaler-ephemeral",
	}
)

type Autoscaler interface {
	// Start runs the autoscaler until cancelled.
	Start(ctx context.Context)

	// Autoscale is used to immediately trigger the autoscaler.
	Autoscale(ctx context.Context)
}

type autoscaler struct {
	statefulSetCache *scheduler.ScaleCache
	statefulSetName  string
	vpodLister       scheduler.VPodLister
	stateAccessor    st.StateAccessor
	trigger          chan context.Context
	evictor          scheduler.Evictor

	// capacity is the total number of virtual replicas available per pod.
	capacity int32

	// refreshPeriod is how often the autoscaler tries to scale down the statefulset
	refreshPeriod time.Duration
	// retryPeriod is how often the autoscaler retry failed autoscale operations
	retryPeriod time.Duration
	lock        sync.Locker

	// isLeader signals whether a given autoscaler instance is leader or not.
	// The autoscaler is considered the leader when ephemeralLeaderElectionObject is in a
	// bucket where we've been promoted.
	isLeader atomic.Bool

	// getReserved returns reserved replicas.
	getReserved GetReserved

	lastCompactAttempt time.Time
}

var (
	_ reconciler.LeaderAware = &autoscaler{}
)

// Promote implements reconciler.LeaderAware.
func (a *autoscaler) Promote(b reconciler.Bucket, _ func(reconciler.Bucket, types.NamespacedName)) error {
	if b.Has(ephemeralLeaderElectionObject) {
		// The promoted bucket has the ephemeralLeaderElectionObject, so we are leader.
		a.isLeader.Store(true)
		// reset the cache to be empty so that when we access state as the leader it is always the newest values
		a.statefulSetCache.Reset()
	}
	return nil
}

// Demote implements reconciler.LeaderAware.
func (a *autoscaler) Demote(b reconciler.Bucket) {
	if b.Has(ephemeralLeaderElectionObject) {
		// The demoted bucket has the ephemeralLeaderElectionObject, so we are not leader anymore.
		a.isLeader.Store(false)
	}
}

func newAutoscaler(cfg *Config, stateAccessor st.StateAccessor, statefulSetCache *scheduler.ScaleCache) *autoscaler {
	a := &autoscaler{
		statefulSetCache: statefulSetCache,
		statefulSetName:  cfg.StatefulSetName,
		vpodLister:       cfg.VPodLister,
		stateAccessor:    stateAccessor,
		evictor:          cfg.Evictor,
		trigger:          make(chan context.Context, 1),
		capacity:         cfg.PodCapacity,
		refreshPeriod:    cfg.RefreshPeriod,
		retryPeriod:      cfg.RetryPeriod,
		lock:             new(sync.Mutex),
		isLeader:         atomic.Bool{},
		getReserved:      cfg.getReserved,
		// Anything that is less than now() - refreshPeriod, so that we will try to compact
		// as soon as we start.
		lastCompactAttempt: time.Now().
			Add(-cfg.RefreshPeriod).
			Add(-time.Minute),
	}

	if a.retryPeriod == 0 {
		a.retryPeriod = time.Second
	}

	return a
}

func (a *autoscaler) Start(ctx context.Context) {
	attemptScaleDown := false
	for {
		autoscaleCtx := ctx
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.refreshPeriod):
			logging.FromContext(ctx).Infow("Triggering scale down", zap.Bool("isLeader", a.isLeader.Load()))
			attemptScaleDown = true
		case autoscaleCtx = <-a.trigger:
			logging.FromContext(autoscaleCtx).Infow("Triggering scale up", zap.Bool("isLeader", a.isLeader.Load()))
			attemptScaleDown = false
		}

		// Retry a few times, just so that we don't have to wait for the next beat when
		// a transient error occurs
		if err := a.syncAutoscale(autoscaleCtx, attemptScaleDown); err != nil {
			logging.FromContext(autoscaleCtx).Errorw("Failed to sync autoscale", zap.Error(err))
			go func() {
				time.Sleep(a.retryPeriod)
				a.Autoscale(ctx) // Use top-level context for background retries
			}()
		}
	}
}

func (a *autoscaler) Autoscale(ctx context.Context) {
	select {
	// We trigger the autoscaler asynchronously by using the channel so that the scale down refresh
	// period is reset.
	case a.trigger <- ctx:
	default:
		// We don't want to block if the channel's buffer is full, it will be triggered eventually.
		logging.FromContext(ctx).Debugw("Skipping autoscale since autoscale is in progress")
	}
}

func (a *autoscaler) syncAutoscale(ctx context.Context, attemptScaleDown bool) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if err := a.doautoscale(ctx, attemptScaleDown); err != nil {
		return fmt.Errorf("failed to do autoscale: %w", err)
	}
	return nil
}

func (a *autoscaler) doautoscale(ctx context.Context, attemptScaleDown bool) error {
	if !a.isLeader.Load() {
		return nil
	}

	logger := logging.FromContext(ctx).With("component", "autoscaler")
	ctx = logging.WithLogger(ctx, logger)

	state, err := a.stateAccessor.State(ctx, a.getReserved())
	if err != nil {
		logger.Info("error while refreshing scheduler state (will retry)", zap.Error(err))
		return err
	}

	scale, err := a.statefulSetCache.GetScale(ctx, a.statefulSetName, metav1.GetOptions{})
	if err != nil {
		// skip a beat
		logger.Infow("failed to get scale subresource", zap.Error(err))
		return err
	}

	logger.Debugw("checking adapter capacity",
		zap.Int32("replicas", scale.Spec.Replicas),
		zap.Any("state", state))

	var scaleUpFactor, newreplicas, minNumPods int32
	scaleUpFactor = 1                                                                                         // Non-HA scaling
	if state.SchedPolicy != nil && contains(nil, state.SchedPolicy.Priorities, st.AvailabilityZonePriority) { //HA scaling across zones
		scaleUpFactor = state.NumZones
	}
	if state.SchedPolicy != nil && contains(nil, state.SchedPolicy.Priorities, st.AvailabilityNodePriority) { //HA scaling across nodes
		scaleUpFactor = state.NumNodes
	}

	newreplicas = state.LastOrdinal + 1 // Ideal number

	if state.SchedulerPolicy == scheduler.MAXFILLUP {
		newreplicas = int32(math.Ceil(float64(state.TotalExpectedVReplicas()) / float64(state.Capacity)))
	} else {
		// Take into account pending replicas and pods that are already filled (for even pod spread)
		pending := state.TotalPending()
		if pending > 0 {
			// Make sure to allocate enough pods for holding all pending replicas.
			if state.SchedPolicy != nil && contains(state.SchedPolicy.Predicates, nil, st.EvenPodSpread) && len(state.FreeCap) > 0 { //HA scaling across pods
				leastNonZeroCapacity := a.minNonZeroInt(state.FreeCap)
				minNumPods = int32(math.Ceil(float64(pending) / float64(leastNonZeroCapacity)))
			} else {
				minNumPods = int32(math.Ceil(float64(pending) / float64(a.capacity)))
			}
			newreplicas += int32(math.Ceil(float64(minNumPods)/float64(scaleUpFactor)) * float64(scaleUpFactor))
		}

		if newreplicas <= state.LastOrdinal {
			// Make sure to never scale down past the last ordinal
			newreplicas = state.LastOrdinal + scaleUpFactor
		}
	}

	// Only scale down if permitted
	if !attemptScaleDown && newreplicas < scale.Spec.Replicas {
		newreplicas = scale.Spec.Replicas
	}

	if newreplicas != scale.Spec.Replicas {
		scale.Spec.Replicas = newreplicas
		logger.Infow("updating adapter replicas", zap.Int32("replicas", scale.Spec.Replicas))

		_, err = a.statefulSetCache.UpdateScale(ctx, a.statefulSetName, scale, metav1.UpdateOptions{})
		if err != nil {
			logger.Errorw("updating scale subresource failed", zap.Error(err))
			return err
		}
	} else if attemptScaleDown {
		// since the number of replicas hasn't changed and time has approached to scale down,
		// take the opportunity to compact the vreplicas
		return a.mayCompact(logger, state, scaleUpFactor)
	}
	return nil
}

func (a *autoscaler) mayCompact(logger *zap.SugaredLogger, s *st.State, scaleUpFactor int32) error {

	// This avoids a too aggressive scale down by adding a "grace period" based on the refresh
	// period
	nextAttempt := a.lastCompactAttempt.Add(a.refreshPeriod)
	if time.Now().Before(nextAttempt) {
		logger.Debugw("Compact was retried before refresh period",
			zap.Time("lastCompactAttempt", a.lastCompactAttempt),
			zap.Time("nextAttempt", nextAttempt),
			zap.String("refreshPeriod", a.refreshPeriod.String()),
		)
		return nil
	}

	logger.Debugw("Trying to compact and scale down",
		zap.Int32("scaleUpFactor", scaleUpFactor),
		zap.Any("state", s),
	)

	// when there is only one pod there is nothing to move or number of pods is just enough!
	if s.LastOrdinal < 1 || len(s.SchedulablePods) <= int(scaleUpFactor) {
		return nil
	}

	if s.SchedulerPolicy == scheduler.MAXFILLUP {
		// Determine if there is enough free capacity to
		// move all vreplicas placed in the last pod to pods with a lower ordinal
		freeCapacity := s.FreeCapacity() - s.Free(s.LastOrdinal)
		usedInLastPod := s.Capacity - s.Free(s.LastOrdinal)

		if freeCapacity >= usedInLastPod {
			a.lastCompactAttempt = time.Now()
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				return fmt.Errorf("vreplicas compaction failed (scaleUpFactor %d): %w", scaleUpFactor, err)
			}
		}

		// only do 1 replica at a time to avoid overloading the scheduler with too many
		// rescheduling requests.
	} else if s.SchedPolicy != nil {
		//Below calculation can be optimized to work for recovery scenarios when nodes/zones are lost due to failure
		freeCapacity := s.FreeCapacity()
		usedInLastXPods := s.Capacity * scaleUpFactor
		for i := int32(0); i < scaleUpFactor && s.LastOrdinal-i >= 0; i++ {
			freeCapacity = freeCapacity - s.Free(s.LastOrdinal-i)
			usedInLastXPods = usedInLastXPods - s.Free(s.LastOrdinal-i)
		}

		if (freeCapacity >= usedInLastXPods) && //remaining pods can hold all vreps from evicted pods
			(s.Replicas-scaleUpFactor >= scaleUpFactor) { //remaining # of pods is enough for HA scaling
			a.lastCompactAttempt = time.Now()
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				return fmt.Errorf("vreplicas compaction failed (scaleUpFactor %d): %w", scaleUpFactor, err)
			}
		}
	}
	return nil
}

func (a *autoscaler) compact(s *st.State, scaleUpFactor int32) error {
	var pod *v1.Pod
	vpods, err := a.vpodLister()
	if err != nil {
		return err
	}

	for _, vpod := range vpods {
		placements := vpod.GetPlacements()
		for i := len(placements) - 1; i >= 0; i-- { //start from the last placement
			for j := int32(0); j < scaleUpFactor; j++ {
				ordinal := st.OrdinalFromPodName(placements[i].PodName)

				if ordinal == s.LastOrdinal-j {
					pod, err = s.PodLister.Get(placements[i].PodName)
					if err != nil {
						return fmt.Errorf("failed to get pod %s: %w", placements[i].PodName, err)
					}

					err = a.evictor(pod, vpod, &placements[i])
					if err != nil {
						return fmt.Errorf("failed to evict pod %s: %w", pod.Name, err)
					}
				}
			}
		}
	}
	return nil
}

func contains(preds []scheduler.PredicatePolicy, priors []scheduler.PriorityPolicy, name string) bool {
	for _, v := range preds {
		if v.Name == name {
			return true
		}
	}
	for _, v := range priors {
		if v.Name == name {
			return true
		}
	}

	return false
}

func (a *autoscaler) minNonZeroInt(slice []int32) int32 {
	min := a.capacity
	for _, v := range slice {
		if v < min && v > 0 {
			min = v
		}
	}
	return min
}
