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

package sinkbinding

import (
	"context"
	"knative.dev/pkg/tracker"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/k8s"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

type CfgFn func(map[string]interface{})

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "sinkbindings"}
}

// Install will create a SinkBinding resource, augmented with the config fn options.
func Install(name string, sink tracker.Reference, subject duckv1.Destination, opts ...CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name":    name,
		"sink":    sink,
		"subject": subject,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a PingSource becomes ready within the time given.
func IsReady(name string, interval, timeout time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, interval, timeout)
}

// WithExtensions adds the ceOVerrides related config to a PingSource spec.
func WithExtensions(extensions map[string]string) CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["ceOverrides"]; !set {
			cfg["ceOverrides"] = map[string]interface{}{}
		}
		ceOverrides := cfg["ceOverrides"].(map[string]interface{})

		if extensions != nil {
			if _, set := ceOverrides["extensions"]; !set {
				ceOverrides["ref"] = map[string]interface{}{}
			}
			ceExt := ceOverrides["extensions"].(map[string]interface{})
			for k, v := range extensions {
				ceExt[k] = v
			}
		}
	}
}
