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
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/statefulset/fake"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	listers "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/eventing/pkg/scheduler"
	"knative.dev/eventing/pkg/scheduler/state"
	tscheduler "knative.dev/eventing/pkg/scheduler/testing"
)

const (
	sfsName       = "statefulset-name"
	vpodName      = "source-name"
	vpodNamespace = "source-namespace"
	numZones      = 3
	numNodes      = 6
)

func TestStatefulsetScheduler(t *testing.T) {
	testCases := []struct {
		name                string
		vreplicas           int32
		replicas            int32
		placements          []duckv1alpha1.Placement
		expected            []duckv1alpha1.Placement
		err                 error
		schedulerPolicyType scheduler.SchedulerPolicyType
		schedulerPolicy     *scheduler.SchedulerPolicy
		deschedulerPolicy   *scheduler.SchedulerPolicy
		pending             map[types.NamespacedName]int32
	}{
		{
			name:                "no replicas, no vreplicas",
			vreplicas:           0,
			replicas:            int32(0),
			expected:            nil,
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:                "no replicas, 1 vreplicas, fail.",
			vreplicas:           1,
			replicas:            int32(0),
			err:                 controller.NewRequeueAfter(5 * time.Second),
			expected:            []duckv1alpha1.Placement{},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:                "one replica, one vreplicas",
			vreplicas:           1,
			replicas:            int32(1),
			expected:            []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:                "one replica, 3 vreplicas",
			vreplicas:           3,
			replicas:            int32(1),
			expected:            []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 3}},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "one replica, 8 vreplicas, already scheduled on unschedulable pod, add replicas",
			vreplicas: 8,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 3},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 8},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "one replica, 1 vreplicas, already scheduled on unschedulable pod, remove replicas",
			vreplicas: 1,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 3},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:                "one replica, 15 vreplicas, unschedulable",
			vreplicas:           15,
			replicas:            int32(1),
			err:                 controller.NewRequeueAfter(5 * time.Second),
			expected:            []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 10}},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "two replicas, 15 vreplicas, scheduled",
			vreplicas: 15,
			replicas:  int32(2),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "two replicas, 15 vreplicas, already scheduled",
			vreplicas: 15,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "two replicas, 20 vreplicas, scheduling",
			vreplicas: 20,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "two replicas, 15 vreplicas, too much scheduled (scale down)",
			vreplicas: 15,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "no replicas, no vreplicas with Predicates and Priorities",
			vreplicas: 0,
			replicas:  int32(0),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "no replicas, 1 vreplicas, fail with Predicates and Priorities",
			vreplicas: 1,
			replicas:  int32(0),
			err:       controller.NewRequeueAfter(5 * time.Second),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "one replica, one vreplicas with Predicates and Priorities",
			vreplicas: 1,
			replicas:  int32(1),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "one replica, 3 vreplicas with Predicates and Priorities",
			vreplicas: 3,
			replicas:  int32(1),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 3}},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "one replica, 15 vreplicas, unschedulable with Predicates and Priorities",
			vreplicas: 15,
			replicas:  int32(1),
			err:       controller.NewRequeueAfter(5 * time.Second),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "two replicas, 12 vreplicas, scheduled with Predicates and no Priorities",
			vreplicas: 12,
			replicas:  int32(2),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 6},
				{PodName: "statefulset-name-1", VReplicas: 6},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
			},
		},
		{
			name:      "two replicas, 15 vreplicas, scheduled with Predicates and Priorities",
			vreplicas: 15,
			replicas:  int32(2),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "two replicas, 15 vreplicas, already scheduled with Predicates and Priorities",
			vreplicas: 15,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "two replicas, 20 vreplicas, scheduling with Predicates and Priorities",
			vreplicas: 20,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 1},
				},
			},
		},
		{
			name:      "no replicas, no vreplicas with two Predicates and two Priorities",
			vreplicas: 0,
			replicas:  int32(0),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread",
						Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "no replicas, 1 vreplicas, fail with two Predicates and two Priorities",
			vreplicas: 1,
			replicas:  int32(0),
			err:       controller.NewRequeueAfter(5 * time.Second),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, one vreplica, with two Predicates and two Priorities (HA)",
			vreplicas: 1,
			replicas:  int32(3),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, three vreplicas, with two Predicates and two Priorities (HA)",
			vreplicas: 3,
			replicas:  int32(3),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "one replica, 15 vreplicas, with two Predicates and two Priorities (HA)",
			vreplicas: 15,
			replicas:  int32(1),
			err:       controller.NewRequeueAfter(5 * time.Second),
			expected:  nil,
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, 15 vreplicas, scheduled, with two Predicates and two Priorities (HA)",
			vreplicas: 15,
			replicas:  int32(3),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, 15 vreplicas, already scheduled, with two Predicates and two Priorities (HA)",
			vreplicas: 15,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 5},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, 30 vreplicas, with two Predicates and two Priorities (HA)",
			vreplicas: 30,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
				{PodName: "statefulset-name-2", VReplicas: 10},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
				{PodName: "statefulset-name-2", VReplicas: 10},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 5}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 5}"},
				},
			},
		},
		{
			name:      "three replicas, 15 vreplicas, with two Predicates and two Priorities (HA)",
			vreplicas: 15,
			replicas:  int32(3),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "three replicas, 20 vreplicas, with two Predicates and two Priorities (HA)",
			vreplicas: 20,
			replicas:  int32(3),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 7},
				{PodName: "statefulset-name-1", VReplicas: 7},
				{PodName: "statefulset-name-2", VReplicas: 6},
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 2}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 2},
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
				},
			},
		},
		{
			name:      "one replica, 8 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 8,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 8},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "two replicas, 15 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 15,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 8},
				{PodName: "statefulset-name-1", VReplicas: 7},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 15 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 15,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithAvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 2 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 2,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 2 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 2,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithAvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 3 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 3,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 2},
				{PodName: "statefulset-name-1", VReplicas: 2},
				{PodName: "statefulset-name-2", VReplicas: 2},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 6 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 7,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 10},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 3},
				{PodName: "statefulset-name-1", VReplicas: 2},
				{PodName: "statefulset-name-2", VReplicas: 2},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "four replicas, 7 vreplicas, too much scheduled (scale down), with two desched Priorities",
			vreplicas: 7,
			replicas:  int32(4),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 4},
				{PodName: "statefulset-name-1", VReplicas: 3},
				{PodName: "statefulset-name-2", VReplicas: 4},
				{PodName: "statefulset-name-3", VReplicas: 3},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 2},
				{PodName: "statefulset-name-1", VReplicas: 2},
				{PodName: "statefulset-name-2", VReplicas: 2},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			deschedulerPolicy: &scheduler.SchedulerPolicy{
				Priorities: []scheduler.PriorityPolicy{
					{Name: "RemoveWithEvenPodSpreadPriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "RemoveWithHighestOrdinalPriority", Weight: 2},
				},
			},
		},
		{
			name:      "three replicas, 15 vreplicas with Predicates and Priorities and non-zero pending for rebalancing",
			vreplicas: 15,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 5},
				{PodName: "statefulset-name-1", VReplicas: 5},
				{PodName: "statefulset-name-2", VReplicas: 5},
			},
			pending: map[types.NamespacedName]int32{{Name: vpodName, Namespace: vpodNamespace}: 5},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 1}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:      "six replicas, one vreplica, with Zone Priority (HA)",
			vreplicas: 1,
			replicas:  int32(6),                                                                //Includes pod/node in unknown zone
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}}, //Not failing the plugin
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:      "six replicas, one vreplica, with Node Priority (HA)",
			vreplicas: 1,
			replicas:  int32(6),                                                                //Includes pod/node in unknown zone
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}}, //Not failing the plugin
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityNodePriority", Weight: 10, Args: "{\"MaxSkew\": 2}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:      "two replicas, 12 vreplicas, already scheduled on overcommitted pod, remove replicas",
			vreplicas: 12,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 12},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
				{PodName: "statefulset-name-1", VReplicas: 2},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:      "one replica, 12 vreplicas, already scheduled on overcommitted pod, remove replicas",
			vreplicas: 12,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 12},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 10},
			},
			err:                 controller.NewRequeueAfter(5 * time.Second),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			nodelist := make([]runtime.Object, 0, numZones)
			podlist := make([]runtime.Object, 0, tc.replicas)
			vpodClient := tscheduler.NewVPodClient()

			for i := int32(0); i < numZones; i++ {
				for j := int32(0); j < numNodes/numZones; j++ {
					nodeName := "node" + fmt.Sprint((j*((numNodes/numZones)+1))+i)
					zoneName := "zone" + fmt.Sprint(i)
					node, err := kubeclient.Get(ctx).CoreV1().Nodes().Create(ctx, tscheduler.MakeNode(nodeName, zoneName), metav1.CreateOptions{})
					if err != nil {
						t.Fatal("unexpected error", err)
					}
					nodelist = append(nodelist, node)
				}
			}
			nodeName := "node" + fmt.Sprint(numNodes) //Node in unknown zone
			node, err := kubeclient.Get(ctx).CoreV1().Nodes().Create(ctx, tscheduler.MakeNodeNoLabel(nodeName), metav1.CreateOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}
			nodelist = append(nodelist, node)

			for i := int32(0); i < tc.replicas; i++ {
				nodeName := "node" + fmt.Sprint(i)
				podName := sfsName + "-" + fmt.Sprint(i)
				pod, err := kubeclient.Get(ctx).CoreV1().Pods(testNs).Create(ctx, tscheduler.MakePod(testNs, podName, nodeName), metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				podlist = append(podlist, pod)
			}

			_, err = kubeclient.Get(ctx).AppsV1().StatefulSets(testNs).Create(ctx, tscheduler.MakeStatefulset(testNs, sfsName, tc.replicas), metav1.CreateOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}
			lsp := listers.NewListers(podlist)
			lsn := listers.NewListers(nodelist)
			scaleCache := scheduler.NewScaleCache(ctx, testNs, kubeclient.Get(ctx).AppsV1().StatefulSets(testNs), scheduler.ScaleCacheConfig{RefreshPeriod: time.Minute * 5})
			sa := state.NewStateBuilder(ctx, testNs, sfsName, vpodClient.List, 10, tc.schedulerPolicyType, tc.schedulerPolicy, tc.deschedulerPolicy, lsp.GetPodLister().Pods(testNs), lsn.GetNodeLister(), scaleCache)
			cfg := &Config{
				StatefulSetNamespace: testNs,
				StatefulSetName:      sfsName,
				VPodLister:           vpodClient.List,
			}
			s := newStatefulSetScheduler(ctx, cfg, sa, nil, lsp.GetPodLister().Pods(testNs))

			// Give some time for the informer to notify the scheduler and set the number of replicas
			err = wait.PollImmediate(200*time.Millisecond, time.Second, func() (bool, error) {
				s.lock.Lock()
				defer s.lock.Unlock()
				return s.replicas == tc.replicas, nil
			})
			if err != nil {
				t.Fatalf("expected number of statefulset replica to be %d (got %d)", tc.replicas, s.replicas)
			}

			vpod := vpodClient.Create(vpodNamespace, vpodName, tc.vreplicas, tc.placements)
			placements, err := s.Schedule(vpod)

			if tc.err == nil && err != nil {
				t.Fatal("unexpected error", err)
			}

			if tc.err != nil && err == nil {
				t.Fatal("expected error, got none")
			}

			if !reflect.DeepEqual(placements, tc.expected) {
				t.Errorf("got %v, want %v", placements, tc.expected)
			}

		})
	}
}

func TestReservePlacements(t *testing.T) {
	testCases := []struct {
		name       string
		vpod       scheduler.VPod
		placements []duckv1alpha1.Placement
		reserved   map[string]int32
	}{
		{
			name:       "no replicas, no placement, no reserved",
			vpod:       tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			placements: nil,
			reserved:   make(map[string]int32),
		},
		{
			name: "one vpod, with placements in 2 pods, no reserved",
			vpod: tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: int32(8)},
				{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			placements: nil,
			reserved:   make(map[string]int32),
		},
		{
			name: "no replicas, new placements, with reserved",
			vpod: tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			reserved: map[string]int32{"statefulset-name-0": 1},
		},
		{
			name: "one vpod, with placements in 2 pods, with reserved",
			vpod: tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: int32(8)},
				{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			reserved: map[string]int32{"statefulset-name-0": 1, "statefulset-name-1": 7},
		},
		{
			name: "one vpod, with placements in 2 pods, with reserved",
			vpod: tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: int32(8)},
				{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
			},
			reserved: map[string]int32{"statefulset-name-0": 1, "statefulset-name-1": 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)

			vpodClient := tscheduler.NewVPodClient()
			vpodClient.Append(tc.vpod)

			cfg := &Config{
				StatefulSetNamespace: testNs,
				StatefulSetName:      sfsName,
				VPodLister:           vpodClient.List,
			}
			fa := newFakeAutoscaler()
			s := newStatefulSetScheduler(ctx, cfg, nil, fa, nil)
			_ = s.Promote(reconciler.UniversalBucket(), func(bucket reconciler.Bucket, name types.NamespacedName) {})

			s.reservePlacements(tc.vpod, tc.vpod.GetPlacements()) //initial reserve

			s.reservePlacements(tc.vpod, tc.placements) //new reserve
			if !reflect.DeepEqual(s.reserved[tc.vpod.GetKey()], tc.reserved) {
				t.Errorf("got %v, want %v", s.reserved[tc.vpod.GetKey()], tc.reserved)
			}

			assert.Equal(t, true, fa.isLeader.Load())

			s.Demote(reconciler.UniversalBucket())
			assert.Equal(t, false, fa.isLeader.Load())
		})
	}
}

type fakeAutoscaler struct {
	isLeader atomic.Bool
}

func (f *fakeAutoscaler) Start(ctx context.Context) {
}

func (f *fakeAutoscaler) Autoscale(ctx context.Context) {
}

func newFakeAutoscaler() *fakeAutoscaler {
	return &fakeAutoscaler{
		isLeader: atomic.Bool{},
	}
}

func (f *fakeAutoscaler) Promote(b reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
	f.isLeader.Store(true)
	return nil
}

func (f *fakeAutoscaler) Demote(bucket reconciler.Bucket) {
	f.isLeader.Store(false)
}

var _ reconciler.LeaderAware = &fakeAutoscaler{}
var _ Autoscaler = &fakeAutoscaler{}
