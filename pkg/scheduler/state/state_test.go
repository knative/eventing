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
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	listers "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/eventing/pkg/scheduler"
	tscheduler "knative.dev/eventing/pkg/scheduler/testing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	testNs   = "test-ns"
	sfsName  = "statefulset-name"
	vpodName = "vpod-name"
	vpodNs   = "vpod-ns"
)

func TestStateBuilder(t *testing.T) {
	testCases := []struct {
		name                string
		replicas            int32
		vpods               [][]duckv1alpha1.Placement
		expected            State
		freec               int32
		schedulerPolicyType scheduler.SchedulerPolicyType
		schedulerPolicy     *scheduler.SchedulerPolicy
		deschedulerPolicy   *scheduler.SchedulerPolicy
		reserved            map[types.NamespacedName]map[string]int32
		nodes               []*v1.Node
		err                 error
	}{
		{
			name:                "no vpods",
			replicas:            int32(0),
			vpods:               [][]duckv1alpha1.Placement{},
			expected:            State{Capacity: 10, FreeCap: []int32{}, SchedulablePods: []int32{}, LastOrdinal: -1, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName},
			freec:               int32(0),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "one vpods",
			replicas: int32(1),
			vpods:    [][]duckv1alpha1.Placement{{{PodName: "statefulset-name-0", VReplicas: 1}}},
			expected: State{Capacity: 10, FreeCap: []int32{int32(9)}, SchedulablePods: []int32{int32(0)}, LastOrdinal: 0, Replicas: 1, NumNodes: 1, NumZones: 1, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
					},
				},
			},
			freec:               int32(9),
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0")},
		},
		{
			name:     "many vpods, no gaps",
			replicas: int32(3),
			vpods: [][]duckv1alpha1.Placement{
				{{PodName: "statefulset-name-0", VReplicas: 1}, {PodName: "statefulset-name-2", VReplicas: 5}},
				{{PodName: "statefulset-name-1", VReplicas: 2}},
				{{PodName: "statefulset-name-1", VReplicas: 3}, {PodName: "statefulset-name-0", VReplicas: 1}},
			},
			expected: State{Capacity: 10, FreeCap: []int32{int32(8), int32(5), int32(5)}, SchedulablePods: []int32{int32(0), int32(1), int32(2)}, LastOrdinal: 2, Replicas: 3, NumNodes: 3, NumZones: 3, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0", "node-1": "zone-1", "node-2": "zone-2"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
						"statefulset-name-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"statefulset-name-1": 2,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"statefulset-name-0": 1,
						"statefulset-name-1": 3,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
						"node-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"node-1": 2,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"node-0": 1,
						"node-1": 3,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
						"zone-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"zone-1": 2,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"zone-0": 1,
						"zone-1": 3,
					},
				},
			},
			freec:               int32(18),
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNode("node-1", "zone-1"), tscheduler.MakeNode("node-2", "zone-2")},
		},
		{
			name:     "many vpods, with gaps",
			replicas: int32(4),
			vpods: [][]duckv1alpha1.Placement{
				{{PodName: "statefulset-name-0", VReplicas: 1}, {PodName: "statefulset-name-2", VReplicas: 5}},
				{{PodName: "statefulset-name-1", VReplicas: 0}},
				{{PodName: "statefulset-name-1", VReplicas: 0}, {PodName: "statefulset-name-3", VReplicas: 0}},
			},
			expected: State{Capacity: 10, FreeCap: []int32{int32(9), int32(10), int32(5), int32(10)}, SchedulablePods: []int32{int32(0), int32(1), int32(2), int32(3)}, LastOrdinal: 2, Replicas: 4, NumNodes: 4, NumZones: 3, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0", "node-1": "zone-1", "node-2": "zone-2", "node-3": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
						"statefulset-name-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"statefulset-name-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"statefulset-name-1": 0,
						"statefulset-name-3": 0,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
						"node-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"node-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"node-1": 0,
						"node-3": 0,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
						"zone-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"zone-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"zone-0": 0,
						"zone-1": 0,
					},
				},
			},
			freec:               int32(34),
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNode("node-1", "zone-1"), tscheduler.MakeNode("node-2", "zone-2"), tscheduler.MakeNode("node-3", "zone-0")},
		},
		{
			name:     "many vpods, with gaps and reserved vreplicas",
			replicas: int32(4),
			vpods: [][]duckv1alpha1.Placement{
				{{PodName: "statefulset-name-0", VReplicas: 1}, {PodName: "statefulset-name-2", VReplicas: 5}},
				{{PodName: "statefulset-name-1", VReplicas: 0}},
				{{PodName: "statefulset-name-1", VReplicas: 0}, {PodName: "statefulset-name-3", VReplicas: 0}},
			},
			expected: State{Capacity: 10, FreeCap: []int32{int32(3), int32(10), int32(5), int32(10)}, SchedulablePods: []int32{int32(0), int32(1), int32(2), int32(3)}, LastOrdinal: 2, Replicas: 4, NumNodes: 4, NumZones: 3, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0", "node-1": "zone-1", "node-2": "zone-2", "node-3": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 2,
						"statefulset-name-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"statefulset-name-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"statefulset-name-1": 0,
						"statefulset-name-3": 0,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 2,
						"node-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"node-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"node-1": 0,
						"node-3": 0,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 2,
						"zone-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"zone-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"zone-0": 0,
						"zone-1": 0,
					},
				},
			},
			freec: int32(28),
			reserved: map[types.NamespacedName]map[string]int32{
				{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
					"statefulset-name-0": 2,
					"statefulset-name-2": 5,
				},
				{Name: vpodName + "-3", Namespace: vpodNs + "-3"}: {
					"statefulset-name-0": 5,
				},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNode("node-1", "zone-1"), tscheduler.MakeNode("node-2", "zone-2"), tscheduler.MakeNode("node-3", "zone-0")},
		},
		{
			name:     "many vpods, with gaps and reserved vreplicas on existing and new placements, fully committed",
			replicas: int32(4),
			vpods: [][]duckv1alpha1.Placement{
				{{PodName: "statefulset-name-0", VReplicas: 1}, {PodName: "statefulset-name-2", VReplicas: 5}},
				{{PodName: "statefulset-name-1", VReplicas: 0}},
				{{PodName: "statefulset-name-1", VReplicas: 0}, {PodName: "statefulset-name-3", VReplicas: 0}},
			},
			expected: State{Capacity: 10, FreeCap: []int32{int32(4), int32(7), int32(5), int32(10), int32(5)}, SchedulablePods: []int32{int32(0), int32(1), int32(2), int32(3)}, LastOrdinal: 4, Replicas: 4, NumNodes: 4, NumZones: 3, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0", "node-1": "zone-1", "node-2": "zone-2", "node-3": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
						"statefulset-name-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"statefulset-name-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"statefulset-name-1": 0,
						"statefulset-name-3": 0,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
						"node-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"node-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"node-1": 0,
						"node-3": 0,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
						"zone-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"zone-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"zone-0": 0,
						"zone-1": 0,
					},
				},
			},
			freec: int32(26),
			reserved: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-3", Namespace: "vpod-ns-3"}: {
					"statefulset-name-4": 5,
				},
				{Name: "vpod-name-4", Namespace: "vpod-ns-4"}: {
					"statefulset-name-0": 5,
					"statefulset-name-1": 3,
				},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNode("node-1", "zone-1"), tscheduler.MakeNode("node-2", "zone-2"), tscheduler.MakeNode("node-3", "zone-0")},
		},
		{
			name:     "many vpods, with gaps and reserved vreplicas on existing and new placements, partially committed",
			replicas: int32(5),
			vpods: [][]duckv1alpha1.Placement{
				{{PodName: "statefulset-name-0", VReplicas: 1}, {PodName: "statefulset-name-2", VReplicas: 5}},
				{{PodName: "statefulset-name-1", VReplicas: 0}},
				{{PodName: "statefulset-name-1", VReplicas: 0}, {PodName: "statefulset-name-3", VReplicas: 0}},
			},
			expected: State{Capacity: 10, FreeCap: []int32{int32(4), int32(7), int32(5), int32(10), int32(2)}, SchedulablePods: []int32{int32(0), int32(1), int32(2), int32(3), int32(4)}, LastOrdinal: 4, Replicas: 5, NumNodes: 5, NumZones: 3, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0", "node-1": "zone-1", "node-2": "zone-2", "node-3": "zone-0", "node-4": "zone-1"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
						"statefulset-name-2": 5,
						"statefulset-name-4": 8,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"statefulset-name-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"statefulset-name-1": 0,
						"statefulset-name-3": 0,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
						"node-2": 5,
						"node-4": 8,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"node-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"node-1": 0,
						"node-3": 0,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
						"zone-1": 8,
						"zone-2": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNs + "-1"}: {
						"zone-1": 0,
					},
					{Name: vpodName + "-2", Namespace: vpodNs + "-2"}: {
						"zone-0": 0,
						"zone-1": 0,
					},
				},
			},
			freec: int32(28),
			reserved: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"statefulset-name-4": 8,
				},
				{Name: "vpod-name-4", Namespace: "vpod-ns-4"}: {
					"statefulset-name-0": 5,
					"statefulset-name-1": 3,
				},
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNode("node-1", "zone-1"), tscheduler.MakeNode("node-2", "zone-2"), tscheduler.MakeNode("node-3", "zone-0"), tscheduler.MakeNode("node-4", "zone-1")},
		},
		{
			name:     "three vpods but one tainted and one wit no zoen label",
			replicas: int32(1),
			vpods:    [][]duckv1alpha1.Placement{{{PodName: "statefulset-name-0", VReplicas: 1}}},
			expected: State{Capacity: 10, FreeCap: []int32{int32(9)}, SchedulablePods: []int32{int32(0)}, LastOrdinal: 0, Replicas: 1, NumNodes: 1, NumZones: 1, SchedulerPolicy: scheduler.MAXFILLUP, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
					},
				},
			},
			freec:               int32(9),
			schedulerPolicyType: scheduler.MAXFILLUP,
			nodes:               []*v1.Node{tscheduler.MakeNode("node-0", "zone-0"), tscheduler.MakeNodeNoLabel("node-1"), tscheduler.MakeNodeTainted("node-2", "zone-2")},
		},
		{
			name:     "one vpod (HA)",
			replicas: int32(1),
			vpods:    [][]duckv1alpha1.Placement{{{PodName: "statefulset-name-0", VReplicas: 1}}},
			expected: State{Capacity: 10, FreeCap: []int32{int32(9)}, SchedulablePods: []int32{int32(0)}, LastOrdinal: 0, Replicas: 1, NumNodes: 1, NumZones: 1, SchedPolicy: &scheduler.SchedulerPolicy{}, DeschedPolicy: &scheduler.SchedulerPolicy{}, StatefulSetName: sfsName,
				NodeToZoneMap: map[string]string{"node-0": "zone-0"},
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"statefulset-name-0": 1,
					},
				},
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"node-0": 1,
					},
				},
				ZoneSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNs + "-0"}: {
						"zone-0": 1,
					},
				},
			},
			freec: int32(9),
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityNodePriority", Weight: 10, Args: "{\"MaxSkew\": 1}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			nodes: []*v1.Node{tscheduler.MakeNode("node-0", "zone-0")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			vpodClient := tscheduler.NewVPodClient()
			nodelist := make([]runtime.Object, 0, len(tc.nodes))
			podlist := make([]runtime.Object, 0, tc.replicas)

			for i, placements := range tc.vpods {
				vpodName := fmt.Sprint(vpodName+"-", i)
				vpodNamespace := fmt.Sprint(vpodNs+"-", i)

				vpodC := vpodClient.Create(vpodNamespace, vpodName, 1, placements)

				lsvp, err := vpodClient.List()
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				vpodG := GetVPod(types.NamespacedName{Name: vpodName, Namespace: vpodNamespace}, lsvp)
				if !reflect.DeepEqual(vpodC, vpodG) {
					t.Errorf("unexpected vpod, got %v, want %v", vpodG, vpodC)
				}
			}

			for i := 0; i < len(tc.nodes); i++ {
				node, err := kubeclient.Get(ctx).CoreV1().Nodes().Create(ctx, tc.nodes[i], metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				nodelist = append(nodelist, node)
			}

			for i := int32(0); i < tc.replicas; i++ {
				nodeName := "node-" + fmt.Sprint(i)
				podName := sfsName + "-" + fmt.Sprint(i)
				pod, err := kubeclient.Get(ctx).CoreV1().Pods(testNs).Create(ctx, tscheduler.MakePod(testNs, podName, nodeName), metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				podlist = append(podlist, pod)
			}

			_, err := kubeclient.Get(ctx).AppsV1().StatefulSets(testNs).Create(ctx, tscheduler.MakeStatefulset(testNs, sfsName, tc.replicas), metav1.CreateOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			lsp := listers.NewListers(podlist)
			lsn := listers.NewListers(nodelist)

			stateBuilder := NewStateBuilder(ctx, testNs, sfsName, vpodClient.List, int32(10), tc.schedulerPolicyType, &scheduler.SchedulerPolicy{}, &scheduler.SchedulerPolicy{}, lsp.GetPodLister().Pods(testNs), lsn.GetNodeLister())
			state, err := stateBuilder.State(tc.reserved)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			tc.expected.PodLister = lsp.GetPodLister().Pods(testNs)
			if tc.expected.FreeCap == nil {
				tc.expected.FreeCap = make([]int32, 0, 256)
			}
			if tc.expected.PodSpread == nil {
				tc.expected.PodSpread = make(map[types.NamespacedName]map[string]int32)
			}
			if tc.expected.NodeSpread == nil {
				tc.expected.NodeSpread = make(map[types.NamespacedName]map[string]int32)
			}
			if tc.expected.ZoneSpread == nil {
				tc.expected.ZoneSpread = make(map[types.NamespacedName]map[string]int32)
			}
			if tc.expected.NodeToZoneMap == nil {
				tc.expected.NodeToZoneMap = make(map[string]string)
			}
			if !reflect.DeepEqual(*state, tc.expected) {
				t.Errorf("unexpected state, got %v, want %v", *state, tc.expected)
			}

			if state.FreeCapacity() != tc.freec {
				t.Errorf("unexpected free capacity, got %d, want %d", state.FreeCapacity(), tc.freec)
			}

			if tc.schedulerPolicy != nil && !SatisfyZoneAvailability(state.SchedulablePods, state) {
				t.Errorf("unexpected state, got %v, want %v", *state, tc.expected)
			}

			if tc.schedulerPolicy != nil && !SatisfyNodeAvailability(state.SchedulablePods, state) {
				t.Errorf("unexpected state, got %v, want %v", *state, tc.expected)
			}
		})
	}
}
