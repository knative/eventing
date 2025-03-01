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
	"context"
	"sync"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/cache"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
)

const (
	// PodAnnotationKey is an annotation used by the scheduler to be informed of pods
	// being evicted and not use it for placing vreplicas
	PodAnnotationKey = "eventing.knative.dev/unschedulable"
)

// VPodLister is the function signature for returning a list of VPods
type VPodLister func() ([]VPod, error)

// Evictor allows for vreplicas to be evicted.
// For instance, the evictor is used by the statefulset scheduler to
// move vreplicas to pod with a lower ordinal.
//
// pod might be `nil`.
type Evictor func(pod *corev1.Pod, vpod VPod, from *duckv1alpha1.Placement) error

// Scheduler is responsible for placing VPods into real Kubernetes pods
type Scheduler interface {
	// Schedule computes the new set of placements for vpod.
	Schedule(ctx context.Context, vpod VPod) ([]duckv1alpha1.Placement, error)
}

// SchedulerFunc type is an adapter to allow the use of
// ordinary functions as Schedulers. If f is a function
// with the appropriate signature, SchedulerFunc(f) is a
// Scheduler that calls f.
type SchedulerFunc func(ctx context.Context, vpod VPod) ([]duckv1alpha1.Placement, error)

// Schedule implements the Scheduler interface.
func (f SchedulerFunc) Schedule(ctx context.Context, vpod VPod) ([]duckv1alpha1.Placement, error) {
	return f(ctx, vpod)
}

// VPod represents virtual replicas placed into real Kubernetes pods
// The scheduler is responsible for placing VPods
type VPod interface {
	GetDeletionTimestamp() *metav1.Time

	// GetKey returns the VPod key (namespace/name).
	GetKey() types.NamespacedName

	// GetVReplicas returns the number of expected virtual replicas
	GetVReplicas() int32

	// GetPlacements returns the current list of placements
	// Do not mutate!
	GetPlacements() []duckv1alpha1.Placement

	GetResourceVersion() string
}

type ScaleClient interface {
	GetScale(ctx context.Context, name string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, name string, scale *autoscalingv1.Scale, options metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

type ScaleCacheConfig struct {
	RefreshPeriod time.Duration `json:"refreshPeriod"`
}

func (c ScaleCacheConfig) withDefaults() ScaleCacheConfig {
	n := ScaleCacheConfig{RefreshPeriod: c.RefreshPeriod}
	if c.RefreshPeriod == 0 {
		n.RefreshPeriod = 5 * time.Minute
	}
	return n
}

type ScaleCache struct {
	entriesMu            sync.RWMutex // protects access to entries, entries itself is concurrency safe, so we only need to ensure that we correctly access the pointer
	entries              *cache.Expiring
	scaleClient          ScaleClient
	statefulSetNamespace string
	config               ScaleCacheConfig
}

type scaleEntry struct {
	specReplicas int32
	statReplicas int32
}

func NewScaleCache(ctx context.Context, namespace string, scaleClient ScaleClient, config ScaleCacheConfig) *ScaleCache {
	return &ScaleCache{
		entries:              cache.NewExpiring(),
		scaleClient:          scaleClient,
		statefulSetNamespace: namespace,
		config:               config.withDefaults(),
	}
}

func (sc *ScaleCache) GetScale(ctx context.Context, statefulSetName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	sc.entriesMu.RLock()
	defer sc.entriesMu.RUnlock()

	if entry, ok := sc.entries.Get(statefulSetName); ok {
		entry := entry.(scaleEntry)
		return &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulSetName,
				Namespace: sc.statefulSetNamespace,
			},
			Spec: autoscalingv1.ScaleSpec{
				Replicas: entry.specReplicas,
			},
			Status: autoscalingv1.ScaleStatus{
				Replicas: entry.statReplicas,
			},
		}, nil
	}

	scale, err := sc.scaleClient.GetScale(ctx, statefulSetName, options)
	if err != nil {
		return scale, err
	}

	sc.setScale(statefulSetName, scale)

	return scale, nil
}

func (sc *ScaleCache) UpdateScale(ctx context.Context, statefulSetName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
	sc.entriesMu.RLock()
	defer sc.entriesMu.RUnlock()

	updatedScale, err := sc.scaleClient.UpdateScale(ctx, statefulSetName, scale, opts)
	if err != nil {
		return updatedScale, err
	}

	sc.setScale(statefulSetName, updatedScale)

	return updatedScale, nil
}

func (sc *ScaleCache) Reset() {
	sc.entriesMu.Lock()
	defer sc.entriesMu.Unlock()

	sc.entries = cache.NewExpiring()
}

func (sc *ScaleCache) setScale(name string, scale *autoscalingv1.Scale) {
	entry := scaleEntry{
		specReplicas: scale.Spec.Replicas,
		statReplicas: scale.Status.Replicas,
	}

	sc.entries.Set(name, entry, sc.config.RefreshPeriod)
}
