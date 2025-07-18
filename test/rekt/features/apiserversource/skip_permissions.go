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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/pkg/adapter/apiserver"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
)

// SkipPermissionsFeature returns a feature testing the skip permissions functionality.
func SkipPermissionsFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource skip permissions")

	source := feature.MakeRandomK8sName("apiserversource")
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
			// Note: We don't set a service account name intentionally
			// to test that the feature works without proper permissions
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

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
		deploymentName := fmt.Sprintf("apiserversource-%s", source)

		obj, err := kubeclient.Get(ctx).AppsV1().Deployments(env.Namespace()).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Deployment not found: %s", err)
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
		t.Logf("ApiServerSource %s is ready with skip permissions enabled", deploymentName)
	})

	f.Stable("ApiServerSource")

	return f
}

// SkipPermissionsDisabledFeature returns a feature testing that skip permissions works when disabled.
func SkipPermissionsDisabledFeature() *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource skip permissions disabled")

	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Install sink service", service.Install(sink, service.WithSelectors(map[string]string{"app": "rekt"})))

	f.Setup("Install ApiServerSource with skip permissions disabled", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(service.AsDestinationRef(sink)),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Pod",
			}),
			apiserversource.WithSkipPermissions(false),
			// Set service account name to default since we're not skipping permissions
			apiserversource.WithServiceAccountName("default"),
		}

		if _, err := apiserversource.InstallLocalYaml(ctx, source, cfg...); err != nil {
			t.Fatal(err)
		}
	})

	f.Assert("ApiServerSource has skip permissions annotation set to false", func(ctx context.Context, t feature.T) {
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

	f.Stable("ApiServerSource")

	return f
}
