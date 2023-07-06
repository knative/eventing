/*
 * Copyright 2023 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"embed"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var templates embed.FS

func GVR() schema.GroupVersionResource {
	return corev1.SchemeGroupVersion.WithResource("services")
}

// Install will create a Service resource. If no ports where defined via the
// WithPorts option, a default mapping 80:8080 will be used.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
		"ports": []corev1.ServicePort{{
			// use the 80:8080 ports by default to be compatible with deprecated
			// resources/svc package
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(8080),
		}},
	}

	for _, fn := range opts {
		fn(cfg)
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, templates, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// AsKReference returns a KReference for a Service without namespace.
func AsKReference(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Service",
		Name:       name,
		APIVersion: "v1",
	}
}

func AsTrackerReference(name string) *tracker.Reference {
	return &tracker.Reference{
		Kind:       "Service",
		Name:       name,
		APIVersion: "v1",
	}
}

func AsDestinationRef(name string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: AsKReference(name),
	}
}

func Address(ctx context.Context, name string) (*duckv1.Addressable, error) {
	return k8s.Address(ctx, GVR(), name)
}
