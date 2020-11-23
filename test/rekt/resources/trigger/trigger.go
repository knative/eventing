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

package trigger

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

type CfgFn func(map[string]interface{})

func gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "triggers"}
}

// WithFilter adds the filter related config to a Trigger spec.
func WithFilter(attributes map[string]string) CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["filter"]; !set {
			cfg["filter"] = map[string]interface{}{}
		}
		filter := cfg["filter"].(map[string]interface{})
		if _, set := filter["filter"]; !set {
			filter["attributes"] = map[string]interface{}{}
		}
		attrs := filter["attributes"].(map[string]interface{})

		for k, v := range attributes {
			attrs[k] = v
		}
	}
}

// WithSubscriber adds the subscriber related config to a Trigger spec.
func WithSubscriber(ref *duckv1.KReference, uri string) CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["subscriber"]; !set {
			cfg["subscriber"] = map[string]interface{}{}
		}
		subscriber := cfg["subscriber"].(map[string]interface{})

		if uri != "" {
			subscriber["uri"] = uri
		}
		if ref != nil {
			if _, set := subscriber["ref"]; !set {
				subscriber["ref"] = map[string]interface{}{}
			}
			sref := subscriber["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			// skip namespace
			sref["name"] = ref.Name
		}
	}
}

// Install will create a Trigger resource, augmented with the config fn options.
func Install(name, brokerName string, opts ...CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name":       name,
		"brokerName": brokerName,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t *testing.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a Trigger becomes ready within the time given.
func IsReady(name string, interval, timeout time.Duration) feature.StepFn {
	return k8s.IsReady(gvr(), name, interval, timeout)
}
