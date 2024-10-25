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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/statefulset/fake"
	"knative.dev/pkg/controller"

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
)

func TestStatefulsetScheduler(t *testing.T) {
	testCases := []struct {
		name             string
		vreplicas        int32
		replicas         int32
		placements       []duckv1alpha1.Placement
		expected         []duckv1alpha1.Placement
		err              error
		pending          map[types.NamespacedName]int32
		initialReserved  map[types.NamespacedName]map[string]int32
		expectedReserved map[types.NamespacedName]map[string]int32
		capacity         int32
	}{
		{
			name:      "no replicas, no vreplicas",
			vreplicas: 0,
			replicas:  int32(0),
			expected:  nil,
		},
		{
			name:      "no replicas, 1 vreplicas, fail.",
			vreplicas: 1,
			replicas:  int32(0),
			err:       controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "one replica, one vreplicas",
			vreplicas: 1,
			replicas:  int32(1),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 1}},
		},
		{
			name:      "one replica, 3 vreplicas",
			vreplicas: 3,
			replicas:  int32(1),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 3}},
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
		},
		{
			name:      "one replica, 15 vreplicas, unschedulable",
			vreplicas: 15,
			replicas:  int32(1),
			err:       controller.NewRequeueAfter(5 * time.Second),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 10}},
		},
		{
			name:      "two replicas, 15 vreplicas, scheduled",
			vreplicas: 15,
			replicas:  int32(2),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 8},
				{PodName: "statefulset-name-1", VReplicas: 7},
			},
		},
		{
			name:      "5 replicas, 4 vreplicas spread, scheduled",
			vreplicas: 4,
			replicas:  int32(5),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
		},
		{
			name:      "2 replicas, 4 vreplicas spread, scheduled",
			vreplicas: 4,
			replicas:  int32(2),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 2},
				{PodName: "statefulset-name-1", VReplicas: 2},
			},
		},
		{
			name:      "3 replicas, 2 new vreplicas spread, scheduled",
			vreplicas: 5,
			replicas:  int32(3),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 2},
				{PodName: "statefulset-name-1", VReplicas: 2},
				{PodName: "statefulset-name-2", VReplicas: 1},
			},
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
			err: controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "with reserved replicas, same vpod",
			vreplicas: 3,
			replicas:  int32(1),
			expected:  []duckv1alpha1.Placement{{PodName: "statefulset-name-0", VReplicas: 3}},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-0": 3,
				},
			},
		},
		{
			name:      "with reserved replicas, different vpod, not enough replicas",
			vreplicas: 3,
			replicas:  int32(1),
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
			},
			err: controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "with reserved replicas, different vpod, not enough replicas and existing placements",
			vreplicas: 3,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
			},
			err: controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "with reserved replicas, different vpod, not enough replicas and existing overcommitted placements",
			vreplicas: 3,
			replicas:  int32(1),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-b", Name: vpodName}: {
					"statefulset-name-0": 5,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-b", Name: vpodName}: {
					"statefulset-name-0": 5,
				},
			},
			err: controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "with reserved replicas, different vpod, with some space",
			vreplicas: 3,
			replicas:  int32(1),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 9,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 9,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-0": 1,
				},
			},
			err: controller.NewRequeueAfter(5 * time.Second),
		},
		{
			name:      "with reserved replicas, different vpod",
			vreplicas: 2,
			replicas:  int32(4),
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-3": 3,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-3": 1,
					"statefulset-name-1": 1,
				},
			},
		},
		{
			name:      "with reserved replicas, same vpod, no op",
			vreplicas: 2,
			replicas:  int32(4),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-3": 1,
					"statefulset-name-1": 1,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-3": 1,
					"statefulset-name-1": 1,
				},
			},
		},
		{
			name:      "with reserved replicas, same vpod, add replica",
			vreplicas: 3,
			replicas:  int32(4),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-1": 1,
					"statefulset-name-3": 1,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-1": 1,
					"statefulset-name-2": 1,
					"statefulset-name-3": 1,
				},
			},
		},
		{
			name:      "with reserved replicas, same vpod, add replicas (replica reserved)",
			vreplicas: 4,
			replicas:  int32(4),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 1},
				{PodName: "statefulset-name-3", VReplicas: 2},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-1": 1,
					"statefulset-name-3": 1,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-1": 1,
					"statefulset-name-2": 1,
					"statefulset-name-3": 2,
				},
			},
		},
		{
			name:      "with reserved replicas, same vpod, remove replicas",
			vreplicas: 1,
			replicas:  int32(4),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
				{PodName: "statefulset-name-1", VReplicas: 1},
				{PodName: "statefulset-name-2", VReplicas: 3},
				{PodName: "statefulset-name-3", VReplicas: 7},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-1", VReplicas: 1},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-0": 1,
					"statefulset-name-1": 1,
					"statefulset-name-2": 3,
					"statefulset-name-3": 7,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 10,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-1": 1,
				},
			},
		},
		{
			name:      "issue",
			vreplicas: 32,
			replicas:  int32(2),
			placements: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 1},
			},
			expected: []duckv1alpha1.Placement{
				{PodName: "statefulset-name-0", VReplicas: 13},
				{PodName: "statefulset-name-1", VReplicas: 19},
			},
			initialReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-b", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-c", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-d", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-e", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-f", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-g", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-0": 1,
				},
			},
			expectedReserved: map[types.NamespacedName]map[string]int32{
				types.NamespacedName{Namespace: vpodNamespace + "-a", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-b", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-c", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-d", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-e", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-f", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace + "-g", Name: vpodName}: {
					"statefulset-name-0": 1,
				},
				types.NamespacedName{Namespace: vpodNamespace, Name: vpodName}: {
					"statefulset-name-0": 13,
					"statefulset-name-1": 19,
				},
			},
			capacity: 20,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			podlist := make([]runtime.Object, 0, tc.replicas)

			vpodClient := tscheduler.NewVPodClient()
			vpod := vpodClient.Create(vpodNamespace, vpodName, tc.vreplicas, tc.placements)

			for i := int32(0); i < tc.replicas; i++ {
				nodeName := "node" + fmt.Sprint(i)
				podName := sfsName + "-" + fmt.Sprint(i)
				pod, err := kubeclient.Get(ctx).CoreV1().Pods(testNs).Create(ctx, tscheduler.MakePod(testNs, podName, nodeName), metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				podlist = append(podlist, pod)
			}

			capacity := int32(0)
			if tc.capacity > 0 {
				capacity = tc.capacity
			}

			_, err := kubeclient.Get(ctx).AppsV1().StatefulSets(testNs).Create(ctx, tscheduler.MakeStatefulset(testNs, sfsName, tc.replicas), metav1.CreateOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}
			lsp := listers.NewListers(podlist)
			scaleCache := scheduler.NewScaleCache(ctx, testNs, kubeclient.Get(ctx).AppsV1().StatefulSets(testNs), scheduler.ScaleCacheConfig{RefreshPeriod: time.Minute * 5})
			sa := state.NewStateBuilder(sfsName, vpodClient.List, capacity, lsp.GetPodLister().Pods(testNs), scaleCache)
			cfg := &Config{
				StatefulSetNamespace: testNs,
				StatefulSetName:      sfsName,
				VPodLister:           vpodClient.List,
			}
			s := newStatefulSetScheduler(ctx, cfg, sa, nil)
			if tc.initialReserved != nil {
				s.reserved = tc.initialReserved
			}

			// Give some time for the informer to notify the scheduler and set the number of replicas
			err = wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, time.Second, true, func(ctx context.Context) (bool, error) {
				s.lock.Lock()
				defer s.lock.Unlock()
				return s.replicas == tc.replicas, nil
			})
			if err != nil {
				t.Fatalf("expected number of statefulset replica to be %d (got %d)", tc.replicas, s.replicas)
			}

			placements, err := s.Schedule(ctx, vpod)

			if tc.err == nil && err != nil {
				t.Fatal("unexpected error", err)
			}

			if tc.err != nil && err == nil {
				t.Fatal("expected error, got none")
			}

			if !reflect.DeepEqual(placements, tc.expected) {
				t.Errorf("placements: got %v, want %v", placements, tc.expected)
			}

			if tc.expectedReserved != nil {
				res := s.Reserved()
				if !reflect.DeepEqual(tc.expectedReserved, res) {
					t.Errorf("expected reserved: got %v, want %v", placements, tc.expected)
				}
			}
		})
	}
}
