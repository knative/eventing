/*
Copyright 2021 The Knative Authors

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

package deployment

import (
	"context"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

// Install will create a Deployment with defaults that can be overwritten by
// the With* methods.
func Install(name string) feature.StepFn {
	cfg := map[string]interface{}{
		"name":      name,
		"selectors": map[string]string{"app": name},         // default
		"image":     "gcr.io/knative-samples/helloworld-go", // default
		"port":      8080,                                   // default
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// AsRef returns a KRef for a Service without namespace.
func AsRef(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
		Name:       name,
	}
}

func AsTrackerReference(name string) *tracker.Reference {
	return &tracker.Reference{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
		Name:       name,
	}
}
