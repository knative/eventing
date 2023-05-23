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

package apiserversource

import (
	"context"
	"embed"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "apiserversources"}
}

// IsReady tests to see if an ApiServerSource becomes ready within the time given.
func IsReady(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timings...)
}

// Install returns a step function which creates an ApiServerSource resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if _, err := InstallLocalYaml(ctx, name, opts...); err != nil {
			t.Error(err)
		}
	}
}

// InstallLocalYaml will create a ApiServerSource resource, augmented with the config fn options.
func InstallLocalYaml(ctx context.Context, name string, opts ...manifest.CfgFn) (manifest.Manifest, error) {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return manifest.InstallYamlFS(ctx, yaml, cfg)
}

// WithServiceAccountName sets the service account name on the ApiServerSource spec.
func WithServiceAccountName(serviceAccountName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["serviceAccountName"] = serviceAccountName
	}
}

// WithEventMode sets the event mode on the ApiServerSource spec.
func WithEventMode(eventMode string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["mode"] = eventMode
	}
}

// WithSink adds the sink related config to a ApiServerSource spec.
func WithSink(d *duckv1.Destination) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["sink"]; !set {
			cfg["sink"] = map[string]interface{}{}
		}
		sink := cfg["sink"].(map[string]interface{})

		ref := d.Ref
		uri := d.URI

		if d.CACerts != nil {
			// This is a multi-line string and should be indented accordingly.
			// Replace "new line" with "new line + spaces".
			sink["CACerts"] = strings.ReplaceAll(*d.CACerts, "\n", "\n      ")
		}

		if uri != nil {
			sink["uri"] = uri.String()
		}
		if ref != nil {
			if _, set := sink["ref"]; !set {
				sink["ref"] = map[string]interface{}{}
			}
			sref := sink["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			sref["namespace"] = ref.Namespace
			sref["name"] = ref.Name
		}
	}
}

// WithResources adds the resources related config to a ApiServerSource spec.
func WithResources(resources ...v1.APIVersionKindSelector) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["resources"]; !set {
			cfg["resources"] = []map[string]interface{}{}
		}

		for _, resource := range resources {
			elem := map[string]interface{}{
				"apiVersion": resource.APIVersion,
				"kind":       resource.Kind,
				"selector":   labelSelectorToStringMap(resource.LabelSelector),
			}
			cfg["resources"] = append(cfg["resources"].([]map[string]interface{}), elem)
		}
	}
}

// WithNamespaceSelector adds a namespace selector to an ApiServerSource spec.
func WithNamespaceSelector(selector *metav1.LabelSelector) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["namespaceSelector"] = labelSelectorToStringMap(selector)
	}
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
