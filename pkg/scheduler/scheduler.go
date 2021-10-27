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
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
)

var (
	ErrNotEnoughReplicas = errors.New("scheduling failed (not enough pod replicas)")
)

type SchedulerPolicyType string

const (
	// MAXFILLUP policy type adds vreplicas to existing pods to fill them up before adding to new pods
	MAXFILLUP SchedulerPolicyType = "MAXFILLUP"

	// PodAnnotationKey is an annotation used by the scheduler to be informed of pods
	// being evicted and not use it for placing vreplicas
	PodAnnotationKey = "eventing.knative.dev/unschedulable"
)

const (
	ZoneLabel = "topology.kubernetes.io/zone"
)

const (
	// MaxWeight is the maximum weight that can be assigned for a priority.
	MaxWeight uint64 = 10
	// MinWeight is the minimum weight that can be assigned for a priority.
	MinWeight uint64 = 0
)

// Policy describes a struct of a policy resource.
type SchedulerPolicy struct {
	// Holds the information to configure the fit predicate functions.
	Predicates []PredicatePolicy
	// Holds the information to configure the priority functions.
	Priorities []PriorityPolicy
}

// PredicatePolicy describes a struct of a predicate policy.
type PredicatePolicy struct {
	// Identifier of the predicate policy
	Name string
	// Holds the parameters to configure the given predicate
	Args interface{}
}

// PriorityPolicy describes a struct of a priority policy.
type PriorityPolicy struct {
	// Identifier of the priority policy
	Name string
	// The numeric multiplier for the pod scores that the priority function generates
	// The weight should be a positive integer
	Weight uint64
	// Holds the parameters to configure the given priority function
	Args interface{}
}

// VPodLister is the function signature for returning a list of VPods
type VPodLister func() ([]VPod, error)

// Evictor allows for vreplicas to be evicted.
// For instance, the evictor is used by the statefulset scheduler to
// move vreplicas to pod with a lower ordinal.
type Evictor func(pod *corev1.Pod, vpod VPod, from *duckv1alpha1.Placement) error

// Scheduler is responsible for placing VPods into real Kubernetes pods
type Scheduler interface {
	// Schedule computes the new set of placements for vpod.
	Schedule(vpod VPod) ([]duckv1alpha1.Placement, error)
}

// VPod represents virtual replicas placed into real Kubernetes pods
// The scheduler is responsible for placing VPods
type VPod interface {
	// GetKey returns the VPod key (namespace/name).
	GetKey() types.NamespacedName

	// GetVReplicas returns the number of expected virtual replicas
	GetVReplicas() int32

	// GetPlacements returns the current list of placements
	// Do not mutate!
	GetPlacements() []duckv1alpha1.Placement

	GetResourceVersion() string
}
