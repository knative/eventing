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
	"embed"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "sinkbindings"}
}

// Install will create a SinkBinding resource, augmented with the config fn options.
func Install(name string, sink *duckv1.Destination, subject *tracker.Reference, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}

	WithSink(sink)(cfg)

	{
		s := map[string]interface{}{}
		if subject != nil {
			s["apiVersion"] = subject.APIVersion
			s["kind"] = subject.Kind
			// skip namespace
			s["name"] = subject.Name
			if subject.Selector != nil {
				// TODO: we are just supporting match labels at the moment.
				s["selectorMatchLabels"] = subject.Selector.MatchLabels
			}
		}
		cfg["subject"] = s
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

// WithExtensions adds the ceOVerrides related config to a PingSource spec.
func WithExtensions(extensions map[string]string) manifest.CfgFn {
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
			// skip namespace
			sref["name"] = ref.Name
		}
	}
}

// IsReady tests to see if a PingSource becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timing...)
}
