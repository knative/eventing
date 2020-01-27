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

package serving

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/reconciler/service"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"

	. "knative.dev/eventing/pkg/reconciler/service/testing"
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
				Name:      "my-svc",
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
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
				WithServingServiceReady(),
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
				Name:      "my-svc",
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
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantStatus: &service.Status{
			IsReady: false,
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
				Name:      "my-svc",
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
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantUpdates: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image2.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantStatus: &service.Status{
			IsReady: false,
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
				Name:      "my-svc",
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
		wantCreates: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantErr: true,
	}, {
		name: "successful update",
		args: service.Args{
			ServiceMeta: metav1.ObjectMeta{
				Name:      "my-svc",
				Namespace: "my-ns",
				Labels:    map[string]string{"app": "test"},
			},
			DeployMeta: metav1.ObjectMeta{
				Name:      "my-svc",
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
			InduceFailure("update", "services"),
		},
		objects: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantUpdates: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image2.example.com", WithPodContainerPort("", 8000))),
			),
		},
		wantErr: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = logging.WithLogger(ctx, logtesting.TestLogger(t))

			ls := NewListers(tc.objects)
			ctx, servingclient := fakeservingclient.With(ctx, ls.GetServingObjects()...)

			for _, reactor := range tc.reactors {
				servingclient.PrependReactor("*", "*", reactor)
			}
			recorderList := ActionRecorderList{servingclient}

			svcReconciler := &ServiceReconciler{
				ServingClientSet: servingclient,
				ServingLister:    ls.GetServingServiceLister(),
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
		name: "successfully get ready status",
		objects: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
				WithServingServiceReady(),
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
		name: "successfully get not ready status",
		objects: []runtime.Object{
			NewServingService("my-svc", "my-ns", "my-svc-rev",
				WithServingServiceOwnerReferences([]metav1.OwnerReference{MakeOwnerReference()}),
				WithServingServiceLabels(map[string]string{"app": "test", "serving.knative.dev/visibility": "cluster-local"}),
				WithServingServiceTemplateLabels(map[string]string{"app": "test"}),
				WithServingServicePodSpec(MakePodSpec("user-container", "image.example.com", WithPodContainerPort("", 8000))),
			),
		},
		svcMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "my-ns",
			Labels:    map[string]string{"app": "test"},
		},
		wantStatus: &service.Status{
			IsReady: false,
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
			ctx, servingclient := fakeservingclient.With(ctx, ls.GetServingObjects()...)

			for _, reactor := range tc.reactors {
				servingclient.PrependReactor("*", "*", reactor)
			}

			svcReconciler := &ServiceReconciler{
				ServingClientSet: servingclient,
				ServingLister:    ls.GetServingServiceLister(),
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

var (
	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)
