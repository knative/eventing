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
	"embed"
	"encoding/json"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"sigs.k8s.io/yaml"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"

	"knative.dev/eventing/test/rekt/resources/delivery"
)

//go:embed *.yaml
var yamlEmbed embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "triggers"}
}

// WithFilter adds the filter related config to a Trigger spec.
func WithFilter(attributes map[string]string) manifest.CfgFn {
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
func WithSubscriber(ref *duckv1.KReference, uri string) manifest.CfgFn {
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

// WithSubscriberFromDestination adds the subscriber related config to a Trigger spec.
func WithSubscriberFromDestination(dest *duckv1.Destination) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["subscriber"]; !set {
			cfg["subscriber"] = map[string]interface{}{}
		}
		subscriber := cfg["subscriber"].(map[string]interface{})

		uri := dest.URI
		ref := dest.Ref

		if dest.CACerts != nil {
			// This is a multi-line string and should be indented accordingly.
			// Replace "new line" with "new line + spaces".
			subscriber["CACerts"] = strings.ReplaceAll(*dest.CACerts, "\n", "\n      ")
		}

		if dest.Audience != nil {
			subscriber["audience"] = *dest.Audience
		}

		if uri != nil {
			subscriber["uri"] = uri.String()
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

// WithAnnotations adds annotations to the trigger
func WithAnnotations(annotations map[string]interface{}) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["annotations"]; !set {
			cfg["annotations"] = map[string]string{}
		}

		if annotations != nil {
			annotation := cfg["annotations"].(map[string]string)
			for k, v := range annotations {
				annotation[k] = v.(string)
			}
		}
	}
}

// WithExtensions adds the ceOverrides related config to a ContainerSource spec.
func WithExtensions(extensions map[string]interface{}) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["ceOverrides"]; !set {
			cfg["ceOverrides"] = map[string]interface{}{}
		}
		ceOverrides := cfg["ceOverrides"].(map[string]interface{})

		if extensions != nil {
			if _, set := ceOverrides["extensions"]; !set {
				ceOverrides["extensions"] = map[string]interface{}{}
			}
			ceExt := ceOverrides["extensions"].(map[string]interface{})
			for k, v := range extensions {
				ceExt[k] = v
			}
		}
	}
}

// WithDeadLetterSink adds the dead letter sink related config to a Trigger spec.
var WithDeadLetterSink = delivery.WithDeadLetterSink

// WithDeadLetterSinkFromDestination adds the dead letter sink related config to the config.
var WithDeadLetterSinkFromDestination = delivery.WithDeadLetterSinkFromDestination

// WithRetry adds the retry related config to a Trigger spec.
var WithRetry = delivery.WithRetry

// WithTimeout adds the timeout related config to the config.
var WithTimeout = delivery.WithTimeout

// Install will create a Trigger resource, augmented with the config fn options.
func Install(name, brokerName string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	if len(brokerName) > 0 {
		cfg["brokerName"] = brokerName
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yamlEmbed, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a Trigger becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

func WithNewFilters(filters []eventingv1.SubscriptionsAPIFilter) manifest.CfgFn {
	jsonBytes, err := json.Marshal(filters)
	if err != nil {
		panic(err)
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		panic(err)
	}

	filtersYaml := string(yamlBytes)

	lines := strings.Split(filtersYaml, "\n")
	out := make([]string, 0, len(lines))
	for i := range lines {
		out = append(out, "    "+lines[i])
	}

	return func(m map[string]interface{}) {
		m["filters"] = strings.Join(out, "\n")
	}
}

func AsKReference(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Trigger",
		Name:       name,
		APIVersion: "eventing.knative.dev/v1",
	}
}
