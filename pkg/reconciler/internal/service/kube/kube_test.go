/*
Copyright 2020 The Knative Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/reconciler/internal/service"
	"knative.dev/pkg/apis"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

func TestReconcile(t *testing.T) {
	cases := []struct {
		name        string
		objects     []runtime.Object
		args        service.Args
		reactors    []clientgotesting.ReactionFunc
		wantCreates []runtime.Object
		wantUpdates []runtime.Object
		wantStatus  *service.Status
		wantErr     bool
	}{{
		name: "already reconciled do nothing",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantStatus: &service.Status{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   "my-svc.my-ns.svc.cluster.local",
			},
		},
	}, {
		name: "successful create",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		wantCreates: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantStatus: &service.Status{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   "my-svc.my-ns.svc.cluster.local",
			},
		},
	}, {
		name: "successful update",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image2.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantUpdates: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image2.example.com", WithPodContainerPort("http", 8000))),
			),
		},
		wantStatus: &service.Status{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   "my-svc.my-ns.svc.cluster.local",
			},
		},
	}, {
		name: "deployment not available",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
				WithDeploymentNotAvailable(),
			),
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantStatus: &service.Status{
			IsReady: false,
			Message: `The Deployment "my-svc-deploy" is unavailable`,
			Reason:  "DeploymentUnavailable",
			URL: &apis.URL{
				Scheme: "http",
				Host:   "my-svc.my-ns.svc.cluster.local",
			},
		},
	}, {
		name: "create deployment error",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		reactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "deployments"),
		},
		wantCreates: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
		},
		wantErr: true,
	}, {
		name: "update deployment error",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image2.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		reactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "deployments"),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
		},
		wantUpdates: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image2.example.com", WithPodContainerPort("http", 8000))),
			),
		},
		wantErr: true,
	}, {
		name: "create service error",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8000),
			),
		},
		reactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "services"),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8000))),
			),
		},
		wantCreates: []runtime.Object{
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantErr: true,
	}, {
		name: "update service error",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc-deploy",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			PodSpec: MakePodSpec(
				"user-container",
				"image.example.com",
				WithPodContainerPort("", 8001),
			),
		},
		reactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "services"),
		},
		objects: []runtime.Object{
			NewDeployment("my-svc-deploy", "my-ns",
				WithDeploymentOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithDeploymentLabels(map[string]string{"app": "test"}),
				WithDeploymentPodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("http", 8001))),
			),
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		wantUpdates: []runtime.Object{
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8001)}}),
			),
		},
		wantErr: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = logging.WithLogger(ctx, logtesting.TestLogger(t))

			ls := NewListers(tc.objects)
			ctx, kubeClient := fakekubeclient.With(ctx, ls.GetKubeObjects()...)

			for _, reactor := range tc.reactors {
				kubeClient.PrependReactor("*", "*", reactor)
			}
			recorderList := ActionRecorderList{kubeClient}

			svcReconciler := &ServiceReconciler{
				KubeClientSet:    kubeClient,
				DeploymentLister: ls.GetDeploymentLister(),
				ServiceLister:    ls.GetServiceLister(),
			}

			status, err := svcReconciler.Reconcile(ctx, MakeOwnerReference(), tc.args)
			if (err != nil) != tc.wantErr {
				t.Error("Service reconcile got err=nil want err")
			}

			if diff := cmp.Diff(tc.wantStatus, status, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Unexpected create (-want, +got): %s", diff)
			}

			actions, err := recorderList.ActionsByVerb()
			if err != nil {
				t.Errorf("Error capturing actions by verb: %q", err)
			}

			for i, want := range tc.wantCreates {
				if i >= len(actions.Creates) {
					t.Errorf("Missing create: %#v", want)
					continue
				}
				got := actions.Creates[i]
				obj := got.GetObject()

				if diff := cmp.Diff(want, obj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("Unexpected create (-want, +got): %s", diff)
				}
			}
			if got, want := len(actions.Creates), len(tc.wantCreates); got > want {
				for _, extra := range actions.Creates[want:] {
					t.Errorf("Extra create: %#v", extra.GetObject())
				}
			}

			for i, want := range tc.wantUpdates {
				if i >= len(actions.Updates) {
					t.Errorf("Missing update: %#v", want)
					continue
				}
				got := actions.Updates[i]
				obj := got.GetObject()

				if diff := cmp.Diff(want, obj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("Unexpected update (-want, +got): %s", diff)
				}
			}
			if got, want := len(actions.Updates), len(tc.wantUpdates); got > want {
				for _, extra := range actions.Updates[want:] {
					t.Errorf("Extra update: %#v", extra.GetObject())
				}
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	cases := []struct {
		name       string
		objects    []runtime.Object
		reactors   []clientgotesting.ReactionFunc
		svcMeta    metav1.ObjectMeta
		wantStatus *service.Status
		wantErr    bool
	}{{
		name: "successfully get status",
		objects: []runtime.Object{
			NewService("my-svc", "my-ns",
				WithServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServiceLabels(map[string]string{"app": "test"}),
				WithServicePorts([]corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000)}}),
			),
		},
		svcMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "my-ns",
			Labels:    map[string]string{"app": "test"},
		},
		wantStatus: &service.Status{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   "my-svc.my-ns.svc.cluster.local",
			},
		},
	}, {
		name: "not found",
		svcMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "my-ns",
			Labels:    map[string]string{"app": "test"},
		},
		wantErr: true,
	}, {
		name: "get service error",
		svcMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "my-ns",
			Labels:    map[string]string{"app": "test"},
		},
		reactors: []clientgotesting.ReactionFunc{
			InduceFailure("get", "services"),
		},
		wantErr: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = logging.WithLogger(ctx, logtesting.TestLogger(t))

			ls := NewListers(tc.objects)
			ctx, kubeClient := fakekubeclient.With(ctx, ls.GetKubeObjects()...)

			for _, reactor := range tc.reactors {
				kubeClient.PrependReactor("*", "*", reactor)
			}

			svcReconciler := &ServiceReconciler{
				KubeClientSet:    kubeClient,
				DeploymentLister: ls.GetDeploymentLister(),
				ServiceLister:    ls.GetServiceLister(),
			}

			status, err := svcReconciler.GetStatus(ctx, tc.svcMeta)
			if (err != nil) != tc.wantErr {
				t.Error("Service reconcile got err=nil want err")
			}
			if diff := cmp.Diff(tc.wantStatus, status, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Unexpected create (-want, +got): %s", diff)
			}
		})
	}
}

func MakeOwnerReference() metav1.OwnerReference {
	trueVal := true
	return metav1.OwnerReference{
		APIVersion:         "test.example.com/v1",
		Kind:               "owner",
		Name:               "owner",
		BlockOwnerDeletion: &trueVal,
		Controller:         &trueVal,
	}
}

type PodSpecOption func(*corev1.PodSpec)

func MakePodSpec(name, image string, opts ...PodSpecOption) corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  name,
				Image: image,
			},
		},
	}

	for _, opt := range opts {
		opt(p)
	}
	return *p
}

func WithPodContainerPort(name string, port int32) PodSpecOption {
	return func(p *corev1.PodSpec) {
		p.Containers[0].Ports = []corev1.ContainerPort{
			{
				Name:          name,
				ContainerPort: port,
			},
		}
	}
}

func WithPodContainerProbes(liveness, readiness *corev1.Probe) PodSpecOption {
	return func(p *corev1.PodSpec) {
		p.Containers[0].LivenessProbe = liveness
		p.Containers[0].ReadinessProbe = readiness
	}
}

var (
	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)
