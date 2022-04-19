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

package testing

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/controller"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/scheduler"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gtesting "k8s.io/client-go/testing"

	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/statefulset/fake"
	rectesting "knative.dev/pkg/reconciler/testing"
)

type sampleVPod struct {
	key         types.NamespacedName
	vreplicas   int32
	placements  []duckv1alpha1.Placement
	rsrcversion string
}

func NewVPod(ns, name string, vreplicas int32, placements []duckv1alpha1.Placement) *sampleVPod {
	return &sampleVPod{
		key: types.NamespacedName{
			Namespace: ns,
			Name:      name,
		},
		vreplicas:   vreplicas,
		placements:  placements,
		rsrcversion: "12345",
	}
}

func (d *sampleVPod) GetKey() types.NamespacedName {
	return d.key
}

func (d *sampleVPod) GetVReplicas() int32 {
	return d.vreplicas
}

func (d *sampleVPod) GetPlacements() []duckv1alpha1.Placement {
	return d.placements
}

func (d *sampleVPod) GetResourceVersion() string {
	return d.rsrcversion
}

func MakeNode(name, zonename string) *v1.Node {
	obj := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				scheduler.ZoneLabel: zonename,
			},
		},
	}
	return obj
}

func MakeNodeNoLabel(name string) *v1.Node {
	obj := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return obj
}

func MakeNodeTainted(name, zonename string) *v1.Node {
	obj := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				scheduler.ZoneLabel: zonename,
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{Key: "node.kubernetes.io/unreachable", Effect: v1.TaintEffectNoExecute},
				{Key: "node.kubernetes.io/unreachable", Effect: v1.TaintEffectNoSchedule},
			},
		},
	}
	return obj
}

func MakeStatefulset(ns, name string, replicas int32) *appsv1.StatefulSet {
	obj := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas: replicas,
		},
	}

	return obj
}

func MakePod(ns, name, nodename string) *v1.Pod {
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.PodSpec{
			NodeName: nodename,
		},
	}
	return obj
}

func SetupFakeContext(t *testing.T) (context.Context, context.CancelFunc) {
	ctx, cancel, informers := rectesting.SetupFakeContextWithCancel(t)
	err := controller.StartInformers(ctx.Done(), informers...)
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	kc := kubeclient.Get(ctx)
	kc.PrependReactor("create", "statefulsets", func(action gtesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(gtesting.CreateActionImpl)
		sfs := createAction.GetObject().(*appsv1.StatefulSet)
		scale := &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sfs.Name,
				Namespace: sfs.Namespace,
			},
			Spec: autoscalingv1.ScaleSpec{
				Replicas: func() int32 {
					if sfs.Spec.Replicas == nil {
						return 1
					}
					return *sfs.Spec.Replicas
				}(),
			},
		}
		kc.Tracker().Add(scale)
		return false, nil, nil
	})

	kc.PrependReactor("get", "statefulsets", func(action gtesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(gtesting.GetAction)
		if action.GetSubresource() == "scale" {
			scale, err := kc.Tracker().Get(autoscalingv1.SchemeGroupVersion.WithResource("scales"), getAction.GetNamespace(), getAction.GetName())
			return true, scale, err

		}
		return false, nil, nil
	})

	kc.PrependReactor("update", "statefulsets", func(action gtesting.Action) (handled bool, ret runtime.Object, err error) {
		updateAction := action.(gtesting.UpdateActionImpl)
		if action.GetSubresource() == "scale" {
			scale := updateAction.GetObject().(*autoscalingv1.Scale)

			err := kc.Tracker().Update(autoscalingv1.SchemeGroupVersion.WithResource("scales"), scale, scale.GetNamespace())
			if err != nil {
				return true, nil, err
			}

			meta, err := meta.Accessor(updateAction.GetObject())
			if err != nil {
				return true, nil, err
			}

			obj, err := kc.Tracker().Get(appsv1.SchemeGroupVersion.WithResource("statefulsets"), meta.GetNamespace(), meta.GetName())
			if err != nil {
				return true, nil, err
			}

			sfs := obj.(*appsv1.StatefulSet)
			sfs.Spec.Replicas = &scale.Spec.Replicas

			err = kc.Tracker().Update(appsv1.SchemeGroupVersion.WithResource("statefulsets"), sfs, sfs.GetNamespace())
			if err != nil {
				return true, nil, err
			}

			return true, scale, nil

		}
		return false, nil, nil
	})

	return ctx, cancel
}
