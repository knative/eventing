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

package svc

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

// Deprecated, use reconciler-test/resources/svc
func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
}

// Install will create a Service resource mapping :80 to :8080 on the provided
// selector for pods.
// Deprecated, use reconciler-test/resources/svc
func Install(name, selectorKey, selectorValue string) feature.StepFn {
	cfg := map[string]interface{}{
		"name":          name,
		"selectorKey":   selectorKey,
		"selectorValue": selectorValue,
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// AsRef returns a KRef for a Service without namespace.
// Deprecated, use reconciler-test/resources/svc
func AsRef(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Service",
		Name:       name,
		APIVersion: "v1",
	}
}

// Deprecated, use reconciler-test/resources/svc
func AsTrackerReference(name string) *tracker.Reference {
	return &tracker.Reference{
		Kind:       "Service",
		Name:       name,
		APIVersion: "v1",
	}
}

// Deprecated, use reconciler-test/resources/svc
func AsDestinationRef(name string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: AsRef(name),
	}
}

// Address
// Deprecated, use reconciler-test/resources/svc
func Address(ctx context.Context, name string) (*apis.URL, error) {
	return k8s.Address(ctx, svc.GVR(), name)
}
