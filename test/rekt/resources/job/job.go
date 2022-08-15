/*
Copyright 2022 The Knative Authors

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

package job

import (
	"context"
	"embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/tracker"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

// Install will create a Job with defaults that can be overwritten by
// the With* methods.
func Install(name string) feature.StepFn {
	cfg := map[string]interface{}{
		"name":  name,
		"image": "gcr.io/knative-nightly/knative.dev/eventing/cmd/heartbeats", // default
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// AsTrackerReference returns a tracker.Reference for a Job without namespace.
func AsTrackerReference(name string) *tracker.Reference {
	return &tracker.Reference{
		Kind:       "Job",
		APIVersion: "batch/v1",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": name,
			},
		},
	}
}
