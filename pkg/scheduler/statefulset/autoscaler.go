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
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"knative.dev/eventing/pkg/scheduler"
	st "knative.dev/eventing/pkg/scheduler/state"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

type Autoscaler interface {
	// Start runs the autoscaler until cancelled.
	Start(ctx context.Context)

	// Autoscale is used to immediately trigger the autoscaler with the hint
	// that pending number of vreplicas couldn't be scheduled.
	Autoscale(ctx context.Context, attemptScaleDown bool, pending int32)
}

type autoscaler struct {
	statefulSetClient clientappsv1.StatefulSetInterface
	statefulSetName   string
	vpodLister        scheduler.VPodLister
	logger            *zap.SugaredLogger
	stateAccessor     st.StateAccessor
	trigger           chan int32
	evictor           scheduler.Evictor

	// capacity is the total number of virtual replicas available per pod.
	capacity int32

	// refreshPeriod is how often the autoscaler tries to scale down the statefulset
	refreshPeriod time.Duration
	lock          sync.Locker
}

func NewAutoscaler(ctx context.Context,
	namespace, name string,
	lister scheduler.VPodLister,
	stateAccessor st.StateAccessor,
	evictor scheduler.Evictor,
	refreshPeriod time.Duration,
	capacity int32) Autoscaler {

	return &autoscaler{
		logger:            logging.FromContext(ctx),
		statefulSetClient: kubeclient.Get(ctx).AppsV1().StatefulSets(namespace),
		statefulSetName:   name,
		vpodLister:        lister,
		stateAccessor:     stateAccessor,
		evictor:           evictor,
		trigger:           make(chan int32, 1),
		capacity:          capacity,
		refreshPeriod:     refreshPeriod,
		lock:              new(sync.Mutex),
	}
}

func (a *autoscaler) Start(ctx context.Context) {
	attemptScaleDown := false
	pending := int32(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.refreshPeriod):
			attemptScaleDown = true
		case pending = <-a.trigger:
			attemptScaleDown = false
		}

		// Retry a few times, just so that we don't have to wait for the next beat when
		// a transient error occurs
		a.syncAutoscale(ctx, attemptScaleDown, pending)
		pending = int32(0)
	}
}

func (a *autoscaler) Autoscale(ctx context.Context, attemptScaleDown bool, pending int32) {
	a.trigger <- pending
	a.syncAutoscale(ctx, attemptScaleDown, pending)
}

func (a *autoscaler) syncAutoscale(ctx context.Context, attemptScaleDown bool, pending int32) {
	a.lock.Lock()
	defer a.lock.Unlock()

	wait.Poll(500*time.Millisecond, 5*time.Second, func() (bool, error) {
		err := a.doautoscale(ctx, attemptScaleDown, pending)
		return err == nil, nil
	})
}

func (a *autoscaler) doautoscale(ctx context.Context, attemptScaleDown bool, pending int32) error {
	state, err := a.stateAccessor.State(nil)
	if err != nil {
		a.logger.Info("error while refreshing scheduler state (will retry)", zap.Error(err))
		return err
	}

	scale, err := a.statefulSetClient.GetScale(ctx, a.statefulSetName, metav1.GetOptions{})
	if err != nil {
		// skip a beat
		a.logger.Infow("failed to get scale subresource", zap.Error(err))
		return err
	}

	a.logger.Infow("checking adapter capacity",
		zap.Int32("pending", pending),
		zap.Int32("replicas", scale.Spec.Replicas),
		zap.Int32("last ordinal", state.LastOrdinal))

	var scaleUpFactor, newreplicas, minNumPods int32
	scaleUpFactor = 1                                                                                         // Non-HA scaling
	if state.SchedPolicy != nil && contains(nil, state.SchedPolicy.Priorities, st.AvailabilityZonePriority) { //HA scaling across zones
		scaleUpFactor = state.NumZones
	}
	if state.SchedPolicy != nil && contains(nil, state.SchedPolicy.Priorities, st.AvailabilityNodePriority) { //HA scaling across nodes
		scaleUpFactor = state.NumNodes
	}

	newreplicas = state.LastOrdinal + 1 // Ideal number

	// Take into account pending replicas and pods that are already filled (for even pod spread)
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

	// Make sure to never scale down past the last ordinal
	if newreplicas <= state.LastOrdinal {
		newreplicas = state.LastOrdinal + scaleUpFactor
	}

	// Only scale down if permitted
	if !attemptScaleDown && newreplicas < scale.Spec.Replicas {
		newreplicas = scale.Spec.Replicas
	}

	if newreplicas != scale.Spec.Replicas {
		scale.Spec.Replicas = newreplicas
		a.logger.Infow("updating adapter replicas", zap.Int32("replicas", scale.Spec.Replicas))

		_, err = a.statefulSetClient.UpdateScale(ctx, a.statefulSetName, scale, metav1.UpdateOptions{})
		if err != nil {
			a.logger.Errorw("updating scale subresource failed", zap.Error(err))
			return err
		}
	} else if attemptScaleDown {
		// since the number of replicas hasn't changed and time has approached to scale down,
		// take the opportunity to compact the vreplicas
		a.mayCompact(state, scaleUpFactor)
	}
	return nil
}

func (a *autoscaler) mayCompact(s *st.State, scaleUpFactor int32) {
	// when there is only one pod there is nothing to move or number of pods is just enough!
	if s.LastOrdinal < 1 || len(s.SchedulablePods) <= int(scaleUpFactor) {
		return
	}

	if s.SchedulerPolicy == scheduler.MAXFILLUP {
		// Determine if there is enough free capacity to
		// move all vreplicas placed in the last pod to pods with a lower ordinal
		freeCapacity := s.FreeCapacity() - s.Free(s.LastOrdinal)
		usedInLastPod := s.Capacity - s.Free(s.LastOrdinal)

		if freeCapacity >= usedInLastPod {
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				a.logger.Errorw("vreplicas compaction failed", zap.Error(err))
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
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				a.logger.Errorw("vreplicas compaction failed", zap.Error(err))
			}
		}
	}
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
					wait.PollImmediate(50*time.Millisecond, 5*time.Second, func() (bool, error) {
						if s.PodLister != nil {
							pod, err = s.PodLister.Get(placements[i].PodName)
						}
						return err == nil, nil
					})

					err = a.evictor(pod, vpod, &placements[i])
					if err != nil {
						return err
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
