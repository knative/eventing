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
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	v1 "k8s.io/client-go/listers/core/v1"
	gtesting "k8s.io/client-go/testing"

	listers "knative.dev/eventing/pkg/reconciler/testing/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/statefulset/fake"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/scheduler"
	"knative.dev/eventing/pkg/scheduler/state"
	st "knative.dev/eventing/pkg/scheduler/state"
	tscheduler "knative.dev/eventing/pkg/scheduler/testing"
)

const (
	testNs = "test-ns"
)

func TestAutoscaler(t *testing.T) {
	testCases := []struct {
		name                string
		replicas            int32
		vpods               []scheduler.VPod
		pendings            int32
		scaleDown           bool
		wantReplicas        int32
		schedulerPolicyType scheduler.SchedulerPolicyType
		schedulerPolicy     *scheduler.SchedulerPolicy
		deschedulerPolicy   *scheduler.SchedulerPolicy
	}{
		{
			name:     "no replicas, no placements, no pending",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			},
			pendings:            int32(0),
			wantReplicas:        int32(0),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "no replicas, no placements, with pending",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 5, nil),
			},
			pendings:            int32(5),
			wantReplicas:        int32(1),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "no replicas, with placements, no pending",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(0),
			wantReplicas:        int32(2),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "no replicas, with placements, with pending, enough capacity",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(3),
			wantReplicas:        int32(3),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "no replicas, with placements, with pending, not enough capacity",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(8),
			wantReplicas:        int32(3),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, no placements, no pending, scale down",
			replicas: int32(3),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			},
			pendings:            int32(0),
			scaleDown:           true,
			wantReplicas:        int32(0),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, no placements, with pending, scale down",
			replicas: int32(3),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 5, nil),
			},
			pendings:            int32(5),
			scaleDown:           true,
			wantReplicas:        int32(1),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, no placements, with pending, scale down disabled",
			replicas: int32(3),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 5, nil),
			},
			pendings:            int32(5),
			scaleDown:           false,
			wantReplicas:        int32(3),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, no placements, with pending, scale up",
			replicas: int32(3),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 5, nil),
			},
			pendings:            int32(40),
			wantReplicas:        int32(4),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, no pending, no change",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(0),
			wantReplicas:        int32(2),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, no pending, scale down",
			replicas: int32(5),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(0),
			scaleDown:           true,
			wantReplicas:        int32(2),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, with pending, enough capacity",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(3),
			wantReplicas:        int32(3),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, with pending, not enough capacity",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:            int32(8),
			wantReplicas:        int32(3),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, no pending, round up capacity",
			replicas: int32(5),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)},
					{PodName: "statefulset-name-2", VReplicas: int32(1)},
					{PodName: "statefulset-name-3", VReplicas: int32(1)},
					{PodName: "statefulset-name-4", VReplicas: int32(1)}}),
			},
			pendings:            int32(0),
			wantReplicas:        int32(5),
			schedulerPolicyType: scheduler.MAXFILLUP,
		},
		{
			name:     "with replicas, with placements, with pending, enough capacity, with Predicates and Zone Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:     int32(3),
			wantReplicas: int32(5),
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityZonePriority", Weight: 10, Args: "{\"MaxSkew\": 1}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:     "with replicas, with placements, with pending, enough capacity, with Predicates and Node Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:     int32(3),
			wantReplicas: int32(8),
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "AvailabilityNodePriority", Weight: 10, Args: "{\"MaxSkew\": 1}"},
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:     "with replicas, with placements, with pending, enough capacity, with Pod Predicates and Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:     int32(3),
			wantReplicas: int32(4),
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
		},
		{
			name:     "with replicas, with placements, with pending, enough capacity, with Pod Predicates and Zone Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:     int32(3),
			wantReplicas: int32(5),
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
			name:     "with replicas, with placements, with pending, enough capacity, with Pod Predicates and Node Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			pendings:     int32(3),
			wantReplicas: int32(8),
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

			for i := int32(0); i < int32(math.Max(float64(tc.wantReplicas), float64(tc.replicas))); i++ {
				nodeName := "node" + fmt.Sprint(i)
				podName := sfsName + "-" + fmt.Sprint(i)
				pod, err := kubeclient.Get(ctx).CoreV1().Pods(testNs).Create(ctx, tscheduler.MakePod(testNs, podName, nodeName), metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				podlist = append(podlist, pod)
			}

			var lspp v1.PodNamespaceLister
			var lsnn v1.NodeLister
			if len(podlist) != 0 {
				lsp := listers.NewListers(podlist)
				lspp = lsp.GetPodLister().Pods(testNs)
			}
			if len(nodelist) != 0 {
				lsn := listers.NewListers(nodelist)
				lsnn = lsn.GetNodeLister()
			}

			stateAccessor := state.NewStateBuilder(ctx, testNs, sfsName, vpodClient.List, 10, tc.schedulerPolicyType, tc.schedulerPolicy, tc.deschedulerPolicy, lspp, lsnn)

			sfsClient := kubeclient.Get(ctx).AppsV1().StatefulSets(testNs)
			_, err := sfsClient.Create(ctx, tscheduler.MakeStatefulset(testNs, sfsName, tc.replicas), metav1.CreateOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			noopEvictor := func(pod *corev1.Pod, vpod scheduler.VPod, from *duckv1alpha1.Placement) error {
				return nil
			}

			autoscaler := NewAutoscaler(ctx, testNs, sfsName, vpodClient.List, stateAccessor, noopEvictor, 10*time.Second, int32(10)).(*autoscaler)

			for _, vpod := range tc.vpods {
				vpodClient.Append(vpod)
			}

			err = autoscaler.doautoscale(ctx, tc.scaleDown, tc.pendings)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			scale, err := sfsClient.GetScale(ctx, sfsName, metav1.GetOptions{})
			if err != nil {
				t.Fatal("unexpected error", err)
			}
			if scale.Spec.Replicas != tc.wantReplicas {
				t.Errorf("unexpected number of replicas, got %d, want %d", scale.Spec.Replicas, tc.wantReplicas)
			}

		})
	}
}

func TestAutoscalerScaleDownToZero(t *testing.T) {
	ctx, cancel := tscheduler.SetupFakeContext(t)

	afterUpdate := make(chan bool)
	kubeclient.Get(ctx).PrependReactor("update", "statefulsets", func(action gtesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "scale" {
			afterUpdate <- true
		}
		return false, nil, nil
	})

	vpodClient := tscheduler.NewVPodClient()
	ls := listers.NewListers(nil)
	stateAccessor := state.NewStateBuilder(ctx, testNs, sfsName, vpodClient.List, 10, scheduler.MAXFILLUP, &scheduler.SchedulerPolicy{}, &scheduler.SchedulerPolicy{}, nil, ls.GetNodeLister())

	sfsClient := kubeclient.Get(ctx).AppsV1().StatefulSets(testNs)
	_, err := sfsClient.Create(ctx, tscheduler.MakeStatefulset(testNs, sfsName, 10), metav1.CreateOptions{})
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	noopEvictor := func(pod *corev1.Pod, vpod scheduler.VPod, from *duckv1alpha1.Placement) error {
		return nil
	}

	autoscaler := NewAutoscaler(ctx, testNs, sfsName, vpodClient.List, stateAccessor, noopEvictor, 2*time.Second, int32(10)).(*autoscaler)

	done := make(chan bool)
	go func() {
		autoscaler.Start(ctx)
		done <- true
	}()

	select {
	case <-afterUpdate:
	case <-time.After(4 * time.Second):
		t.Fatal("timeout waiting for scale subresource to be updated")

	}

	sfs, err := sfsClient.Get(ctx, sfsName, metav1.GetOptions{})
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	if *sfs.Spec.Replicas != 0 {
		t.Errorf("unexpected number of replicas, got %d, want 3", *sfs.Spec.Replicas)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for autoscaler to stop")
	}
}

func TestCompactor(t *testing.T) {
	testCases := []struct {
		name                string
		replicas            int32
		vpods               []scheduler.VPod
		schedulerPolicyType scheduler.SchedulerPolicyType
		wantEvictions       map[types.NamespacedName][]duckv1alpha1.Placement
		schedulerPolicy     *scheduler.SchedulerPolicy
		deschedulerPolicy   *scheduler.SchedulerPolicy
	}{
		{
			name:     "no replicas, no placements, no pending",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions:       nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, compacted",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions:       nil,
		},
		{
			name:     "one vpod, with  placements in 2 pods, compacted edge",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 11, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(3)}}),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions:       nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, not compacted",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 10, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(2)}}),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions: map[types.NamespacedName][]duckv1alpha1.Placement{
				{Name: "vpod-1", Namespace: testNs}: {{PodName: "statefulset-name-1", VReplicas: int32(2)}},
			},
		},
		{
			name:     "multiple vpods, with placements in multiple pods, compacted",
			replicas: int32(3),
			// pod-0:6, pod-1:8, pod-2:7
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 12, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(8)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 9, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(7)}}),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions:       nil,
		},
		{
			name:     "multiple vpods, with placements in multiple pods, not compacted",
			replicas: int32(3),
			// pod-0:6, pod-1:7, pod-2:7
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 6, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(7)}}),
			},
			schedulerPolicyType: scheduler.MAXFILLUP,
			wantEvictions: map[types.NamespacedName][]duckv1alpha1.Placement{
				{Name: "vpod-2", Namespace: testNs}: {{PodName: "statefulset-name-2", VReplicas: int32(7)}},
			},
		},
		{
			name:     "no replicas, no placements, no pending, with Predicates and Priorities",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, compacted, with Predicates and Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with  placements in 2 pods, compacted edge, with Predicates and Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 11, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(3)}}),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, not compacted, with Predicates and Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 10, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(2)}}),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: map[types.NamespacedName][]duckv1alpha1.Placement{
				{Name: "vpod-1", Namespace: testNs}: {{PodName: "statefulset-name-1", VReplicas: int32(2)}},
			},
		},
		{
			name:     "multiple vpods, with placements in multiple pods, compacted, with Predicates and Priorities",
			replicas: int32(3),
			// pod-0:6, pod-1:8, pod-2:7
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 12, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(8)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 9, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(7)}}),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: nil,
		},
		{
			name:     "multiple vpods, with placements in multiple pods, not compacted, with Predicates and Priorities",
			replicas: int32(3),
			// pod-0:6, pod-1:7, pod-2:7
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 6, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(7)}}),
			},
			schedulerPolicy: &scheduler.SchedulerPolicy{
				Predicates: []scheduler.PredicatePolicy{
					{Name: "PodFitsResources"},
					{Name: "EvenPodSpread", Args: "{\"MaxSkew\": 1}"},
				},
				Priorities: []scheduler.PriorityPolicy{
					{Name: "LowestOrdinalPriority", Weight: 5},
				},
			},
			wantEvictions: map[types.NamespacedName][]duckv1alpha1.Placement{
				{Name: "vpod-2", Namespace: testNs}: {{PodName: "statefulset-name-2", VReplicas: int32(7)}},
			},
		},
		{
			name:     "no replicas, no placements, no pending, with Predicates and HA Priorities",
			replicas: int32(0),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 0, nil),
			},
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
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, compacted, with Predicates and HA Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 15, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(7)}}),
			},
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
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with placements in 2 pods, compacted edge, with Predicates and HA Priorities",
			replicas: int32(2),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 11, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(3)}}),
			},
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
			wantEvictions: nil,
		},
		{
			name:     "one vpod, with placements in 3 pods, compacted, with Predicates and HA Priorities",
			replicas: int32(3),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 14, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(8)},
					{PodName: "statefulset-name-1", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(4)}}),
			},
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
			wantEvictions: nil,
		},
		{
			name:     "multiple vpods, with placements in multiple pods, compacted, with Predicates and HA Priorities",
			replicas: int32(3),
			// pod-0:6, pod-1:8, pod-2:7
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 12, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(8)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 9, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(7)}}),
			},
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
			wantEvictions: nil,
		},
		{
			name:     "multiple vpods, with placements in multiple pods, not compacted, with Predicates and HA Priorities",
			replicas: int32(6),
			vpods: []scheduler.VPod{
				tscheduler.NewVPod(testNs, "vpod-1", 16, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(4)},
					{PodName: "statefulset-name-1", VReplicas: int32(2)},
					{PodName: "statefulset-name-2", VReplicas: int32(2)},
					{PodName: "statefulset-name-3", VReplicas: int32(2)},
					{PodName: "statefulset-name-4", VReplicas: int32(3)},
					{PodName: "statefulset-name-5", VReplicas: int32(3)}}),
				tscheduler.NewVPod(testNs, "vpod-2", 11, []duckv1alpha1.Placement{
					{PodName: "statefulset-name-0", VReplicas: int32(2)},
					{PodName: "statefulset-name-1", VReplicas: int32(4)},
					{PodName: "statefulset-name-2", VReplicas: int32(5)}}),
			},
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
			wantEvictions: map[types.NamespacedName][]duckv1alpha1.Placement{
				{Name: "vpod-1", Namespace: testNs}: {{PodName: "statefulset-name-5", VReplicas: int32(3)}, {PodName: "statefulset-name-4", VReplicas: int32(3)}, {PodName: "statefulset-name-3", VReplicas: int32(2)}},
			},
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
			for i := int32(0); i < tc.replicas; i++ {
				nodeName := "node" + fmt.Sprint(i)
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
			stateAccessor := state.NewStateBuilder(ctx, testNs, sfsName, vpodClient.List, 10, tc.schedulerPolicyType, tc.schedulerPolicy, tc.deschedulerPolicy, lsp.GetPodLister().Pods(testNs), lsn.GetNodeLister())

			evictions := make(map[types.NamespacedName][]duckv1alpha1.Placement)
			recordEviction := func(pod *corev1.Pod, vpod scheduler.VPod, from *duckv1alpha1.Placement) error {
				evictions[vpod.GetKey()] = append(evictions[vpod.GetKey()], *from)
				return nil
			}

			autoscaler := NewAutoscaler(ctx, testNs, sfsName, vpodClient.List, stateAccessor, recordEviction, 10*time.Second, int32(10)).(*autoscaler)

			for _, vpod := range tc.vpods {
				vpodClient.Append(vpod)
			}

			state, err := stateAccessor.State(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var scaleUpFactor int32
			if tc.schedulerPolicy != nil && contains(nil, tc.schedulerPolicy.Priorities, st.AvailabilityZonePriority) { //HA scaling across zones
				scaleUpFactor = state.NumZones
			} else if tc.schedulerPolicy != nil && contains(nil, tc.schedulerPolicy.Priorities, st.AvailabilityNodePriority) { //HA scalingacross nodes
				scaleUpFactor = state.NumNodes
			} else {
				scaleUpFactor = 1 // Non-HA scaling
			}

			autoscaler.mayCompact(state, scaleUpFactor)

			if tc.wantEvictions == nil && len(evictions) != 0 {
				t.Fatalf("unexpected evictions: %v", evictions)

			}
			for key, placements := range tc.wantEvictions {
				got, ok := evictions[key]
				if !ok {
					t.Fatalf("unexpected %v to be evicted but was not", key)
				}

				if !reflect.DeepEqual(placements, got) {
					t.Fatalf("expected evicted placement to be %v, but got %v", placements, got)
				}

				delete(evictions, key)
			}

			if len(evictions) != 0 {
				t.Fatalf("unexpected evictions %v", evictions)
			}
		})
	}
}
