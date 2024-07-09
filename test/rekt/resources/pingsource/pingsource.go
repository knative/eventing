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

package pingsource

import (
	"context"
	"embed"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/k8s"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "pingsources"}
}

// Install will create a Broker resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err, cfg)
		}
	}
}

// IsReady tests to see if a PingSource becomes ready within the time given.
func IsReady(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timings...)
}

// WithSink adds the sink related config to a PingSource spec.
func WithSink(dest *duckv1.Destination) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["sink"]; !set {
			cfg["sink"] = map[string]interface{}{}
		}
		sink := cfg["sink"].(map[string]interface{})

		uri := dest.URI
		ref := dest.Ref

		if dest.CACerts != nil {
			// This is a multi-line string and should be indented accordingly.
			// Replace "new line" with "new line + spaces".
			sink["CACerts"] = strings.ReplaceAll(*dest.CACerts, "\n", "\n      ")
		}
		if dest.Audience != nil {
			sink["audience"] = *dest.Audience
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
			if ref.Namespace != "" {
				sref["namespace"] = ref.Namespace
			}
			sref["name"] = ref.Name
		}

	}
}

// WithData adds the contentType and data config to a PingSource spec.
func WithData(contentType, data string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if contentType != "" {
			cfg["contentType"] = contentType
		}
		if data != "" {
			cfg["data"] = data
		}
	}
}

// WithDataBase64 adds the contentType and dataBase64 config to a PingSource spec.
func WithDataBase64(contentType, dataBase64 string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if contentType != "" {
			cfg["contentType"] = contentType
		}
		if dataBase64 != "" {
			cfg["dataBase64"] = dataBase64
		}
	}
}

// WithSchedule add the schedule config to a Pingsource spec.
func WithSchedule(schedule string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if schedule != "" {
			cfg["schedule"] = schedule
		}
	}
}
