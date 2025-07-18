/*
Copyright 2025 The Knative Authors

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

package apiserversource

import (
	"context"
	"encoding/json"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/pkg/adapter/apiserver"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
)

func SkipPermissionsFeature() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Features - Permissions Check",
		Features: []*feature.Feature{
			SkipPermissionsEnabledNoRBACFeature(),
			SkipPermissionsEnabledWithRBACFeature(),
			SkipPermissionsDisabledNoRBACFeature(),
			SkipPermissionsDisabledWithRBACFeature(),
		},
	}
	return fs
}

// SkipPermissionsFeature returns a feature testing the skip permissions functionality.
func SkipPermissionsEnabledNoRBACFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource skip permissions but pod has no permissions")

	source := feature.MakeRandomK8sName("one")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Install sink service", service.Install(sink, service.WithSelectors(map[string]string{"app": "rekt"})))

	f.Setup("Install ApiServerSource with skip permissions", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(service.AsDestinationRef(sink)),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Pod",
			}),
			apiserversource.WithSkipPermissions(true),
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Requirement("ApiServerSource is not ready", k8s.IsNotReady(apiserversource.Gvr(), source))

	f.Assert("ApiServerSource has skip permissions annotation", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		obj, err := eventingclient.Get(ctx).SourcesV1().ApiServerSources(env.Namespace()).Get(ctx, source, metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get ApiServerSource:", err)
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			t.Fatal("ApiServerSource has no annotations")
		}

		skipPermissions, found := annotations["features.knative.dev/apiserversource-skip-permissions-check"]
		if !found {
			t.Fatal("Skip permissions annotation not found")
		}

		if skipPermissions != "true" {
			t.Fatalf("Expected skip permissions annotation to be 'true', got: %s", skipPermissions)
		}
	})

	f.Assert("ApiServerSource adapter has FailFast set to true", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		objs, err := kubeclient.Get(ctx).AppsV1().Deployments(env.Namespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Could not list Deployment: %s", err)
		}

		var obj *appsv1.Deployment
		for _, deployment := range objs.Items {
			if strings.HasPrefix(deployment.Name, "apiserversource-one-") {
				obj = &deployment
				break
			}
		}

		if obj == nil {
			t.Fatal("could not found a Deployment prefixed with 'apiserversource-one-'")
		}

		for _, envvar := range obj.Spec.Template.Spec.Containers[0].Env {
			if envvar.Name == "K_SOURCE_CONFIG" {
				var config apiserver.Config
				err := json.Unmarshal([]byte(envvar.Value), &config)
				if err != nil {
					t.Fatalf("error deserializing K_SOURCE_CONFIG json: %s", err)
				}

				if !config.FailFast {
					t.Fatal("failFast should be true")
				}

			}
		}
		t.Logf("ApiServerSource %s has skip permissions enabled", obj.Name)
	})

	f.Stable("ApiServerSource")

	return f
}

// SkipPermissionsEnabledWithRBACFeature tests
func SkipPermissionsEnabledWithRBACFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource skip permissions and pod has permissions")

	source := feature.MakeRandomK8sName("two")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Create RBAC resources", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		_, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(env.Namespace()).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "two",
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create ServiceAccount:", err)
		}

		_, err = kubeclient.Get(ctx).RbacV1().Roles(env.Namespace()).Create(ctx, &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-pods-two",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create Role:", err)
		}

		_, err = kubeclient.Get(ctx).RbacV1().RoleBindings(env.Namespace()).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-pods-two",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "two",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "get-pods-two",
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create RoleBinding:", err)
		}
	})

	f.Setup("Install sink service", service.Install(sink, service.WithSelectors(map[string]string{"app": "rekt"})))

	f.Setup("Install ApiServerSource with skip permissions", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(service.AsDestinationRef(sink)),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Pod",
			}),
			apiserversource.WithSkipPermissions(true),
			apiserversource.WithServiceAccountName("two"),
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Requirement("ApiServerSource is ready", apiserversource.IsReady(source))

	f.Assert("ApiServerSource has skip permissions annotation", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		obj, err := eventingclient.Get(ctx).SourcesV1().ApiServerSources(env.Namespace()).Get(ctx, source, metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get ApiServerSource:", err)
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			t.Fatal("ApiServerSource has no annotations")
		}

		skipPermissions, found := annotations["features.knative.dev/apiserversource-skip-permissions-check"]
		if !found {
			t.Fatal("Skip permissions annotation not found")
		}

		if skipPermissions != "true" {
			t.Fatalf("Expected skip permissions annotation to be 'true', got: %s", skipPermissions)
		}
	})

	f.Assert("ApiServerSource adapter has FailFast set to true", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		objs, err := kubeclient.Get(ctx).AppsV1().Deployments(env.Namespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Could not list Deployment: %s", err)
		}

		var obj *appsv1.Deployment
		for _, deployment := range objs.Items {
			if strings.HasPrefix(deployment.Name, "apiserversource-two-") {
				obj = &deployment
				break
			}
		}

		if obj == nil {
			t.Fatal("could not found a Deployment prefixed with 'apiserversource-two-'")
		}

		if obj.Status.ReadyReplicas != 1 {
			t.Fatalf("Expected 1 ready replica, got: %d", obj.Status.ReadyReplicas)
		}

		if obj.Status.UnavailableReplicas != 0 {
			t.Fatalf("Expected 0 unavailable replica, got: %d", obj.Status.UnavailableReplicas)
		}

		for _, envvar := range obj.Spec.Template.Spec.Containers[0].Env {
			if envvar.Name == "K_SOURCE_CONFIG" {
				var config apiserver.Config
				err := json.Unmarshal([]byte(envvar.Value), &config)
				if err != nil {
					t.Fatalf("error deserializing K_SOURCE_CONFIG json: %s", err)
				}

				if !config.FailFast {
					t.Fatal("failFast should be true")
				}

			}
		}
		t.Logf("ApiServerSource %s has skip permissions enabled", obj.Name)
	})

	f.Stable("ApiServerSource")

	return f
}

// SkipPermissionsFeature returns a feature testing the skip permissions functionality.
func SkipPermissionsDisabledNoRBACFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource checks permissions but there are no permissions")

	source := feature.MakeRandomK8sName("three")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Install sink service", service.Install(sink, service.WithSelectors(map[string]string{"app": "rekt"})))

	f.Setup("Install ApiServerSource with skip permissions", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(service.AsDestinationRef(sink)),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Pod",
			}),
			apiserversource.WithSkipPermissions(false),
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Requirement("ApiServerSource is not ready", k8s.IsNotReady(apiserversource.Gvr(), source))

	f.Assert("ApiServerSource has no skip permissions annotation", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		obj, err := eventingclient.Get(ctx).SourcesV1().ApiServerSources(env.Namespace()).Get(ctx, source, metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get ApiServerSource:", err)
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			t.Fatal("ApiServerSource has no annotations")
		}

		skipPermissions, found := annotations["features.knative.dev/apiserversource-skip-permissions-check"]
		if !found {
			t.Fatal("Skip permissions annotation not found")
		}

		if skipPermissions != "false" {
			t.Fatalf("Expected skip permissions annotation to be 'false', got: %s", skipPermissions)
		}
	})

	f.Assert("ApiServerSource adapter has FailFast set to false", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		objs, err := kubeclient.Get(ctx).AppsV1().Deployments(env.Namespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Could not list Deployment: %s", err)
		}

		var obj *appsv1.Deployment
		for _, deployment := range objs.Items {
			if strings.HasPrefix(deployment.Name, "apiserversource-three-") {
				obj = &deployment
				break
			}
		}

		if obj != nil {
			t.Fatal("found a Deployment prefixed with 'apiserversource-three-' but expected none")
		}
	})

	f.Stable("ApiServerSource")

	return f
}

// SkipPermissionsEnabledWithRBACFeature tests
func SkipPermissionsDisabledWithRBACFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource does not skip permissions and pod has permissions")

	source := feature.MakeRandomK8sName("four")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Create RBAC resources", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		_, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(env.Namespace()).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "four",
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create ServiceAccount:", err)
		}

		_, err = kubeclient.Get(ctx).RbacV1().Roles(env.Namespace()).Create(ctx, &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-pods-four",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					Resources: []string{"pods"},
					APIGroups: []string{""},
				},
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create Role:", err)
		}

		_, err = kubeclient.Get(ctx).RbacV1().RoleBindings(env.Namespace()).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-pods-four",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "four",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "get-pods-four",
			},
		}, metav1.CreateOptions{})

		if err != nil {
			t.Fatal("Failed to create RoleBinding:", err)
		}
	})

	f.Setup("Install sink service", service.Install(sink, service.WithSelectors(map[string]string{"app": "rekt"})))

	f.Setup("Install ApiServerSource with skip permissions", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(service.AsDestinationRef(sink)),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Pod",
			}),
			apiserversource.WithSkipPermissions(false),
			apiserversource.WithServiceAccountName("four"),
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Requirement("ApiServerSource is ready", apiserversource.IsReady(source))

	f.Assert("ApiServerSource has skip permissions annotation", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		obj, err := eventingclient.Get(ctx).SourcesV1().ApiServerSources(env.Namespace()).Get(ctx, source, metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get ApiServerSource:", err)
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			t.Fatal("ApiServerSource has no annotations")
		}

		skipPermissions, found := annotations["features.knative.dev/apiserversource-skip-permissions-check"]
		if !found {
			t.Fatal("Skip permissions annotation not found")
		}

		if skipPermissions != "false" {
			t.Fatalf("Expected skip permissions annotation to be 'false', got: %s", skipPermissions)
		}
	})

	f.Assert("ApiServerSource adapter has FailFast set to false", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		objs, err := kubeclient.Get(ctx).AppsV1().Deployments(env.Namespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Could not list Deployment: %s", err)
		}

		var obj *appsv1.Deployment
		for _, deployment := range objs.Items {
			if strings.HasPrefix(deployment.Name, "apiserversource-four-") {
				obj = &deployment
				break
			}
		}

		if obj == nil {
			t.Fatal("could not found a Deployment prefixed with 'apiserversource-four-'")
		}

		if obj.Status.ReadyReplicas != 1 {
			t.Fatalf("Expected 1 ready replica, got: %d", obj.Status.ReadyReplicas)
		}

		if obj.Status.UnavailableReplicas != 0 {
			t.Fatalf("Expected 0 unavailable replica, got: %d", obj.Status.UnavailableReplicas)
		}

		for _, envvar := range obj.Spec.Template.Spec.Containers[0].Env {
			if envvar.Name == "K_SOURCE_CONFIG" {
				var config apiserver.Config
				err := json.Unmarshal([]byte(envvar.Value), &config)
				if err != nil {
					t.Fatalf("error deserializing K_SOURCE_CONFIG json: %s", err)
				}

				if config.FailFast {
					t.Fatal("failFast should be false")
				}
			}
		}
		t.Logf("ApiServerSource %s has skip permissions enabled", obj.Name)
	})

	f.Stable("ApiServerSource")

	return f
}
