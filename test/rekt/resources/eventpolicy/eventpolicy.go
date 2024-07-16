/*
Copyright 2024 The Knative Authors

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

package eventpolicy

import (
	"context"
	"embed"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1alpha1", Resource: "eventpolicies"}
}

// Install will create an EventPolicy resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func WithTo(tos ...eventingv1alpha1.EventPolicySpecTo) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["to"]; !set {
			cfg["to"] = []map[string]interface{}{}
		}

		res := cfg["to"].([]map[string]interface{})
		for _, ref := range tos {
			to := map[string]interface{}{}
			if ref.Ref != nil {
				to = map[string]interface{}{
					"ref": map[string]interface{}{
						"apiVersion": ref.Ref.APIVersion,
						"kind":       ref.Ref.Kind,
						"name":       ref.Ref.Name,
					}}
			}

			if ref.Selector != nil {
				selector := labelSelectorToStringMap(ref.Selector.LabelSelector)
				selector["apiVersion"] = ref.Selector.APIVersion
				selector["kind"] = ref.Selector.Kind

				to = map[string]interface{}{
					"selector": selector,
				}
			}

			res = append(res, to)
		}

		cfg["to"] = res
	}
}

func WithFrom(froms ...eventingv1alpha1.EventPolicySpecFrom) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["from"]; !set {
			cfg["from"] = []map[string]interface{}{}
		}

		res := cfg["from"].([]map[string]interface{})
		for _, ref := range froms {
			from := map[string]interface{}{}
			if ref.Ref != nil {
				from = map[string]interface{}{
					"ref": map[string]interface{}{
						"apiVersion": ref.Ref.APIVersion,
						"kind":       ref.Ref.Kind,
						"name":       ref.Ref.Name,
						"namespace":  ref.Ref.Namespace,
					}}
			}

			if ref.Sub != nil && *ref.Sub != "" {
				from = map[string]interface{}{
					"sub": *ref.Sub,
				}
			}

			res = append(res, from)
		}

		cfg["from"] = res
	}
}

// IsReady tests to see if an EventPolicy becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

func labelSelectorToStringMap(selector *metav1.LabelSelector) map[string]interface{} {
	if selector == nil {
		return nil
	}

	r := map[string]interface{}{}

	r["matchLabels"] = selector.MatchLabels

	if selector.MatchExpressions != nil {
		me := []map[string]interface{}{}
		for _, ml := range selector.MatchExpressions {
			me = append(me, map[string]interface{}{
				"key":      ml.Key,
				"operator": ml.Operator,
				"values":   ml.Values,
			})
		}
		r["matchExpressions"] = me
	}

	return r
}
