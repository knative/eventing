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

package integrationsource

import (
	"context"
	"embed"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed integrationsource.yaml
var yaml embed.FS

type SourceType string

const (
	SourceTypeTimer SourceType = "dev.knative.eventing.timer"
	SourceTypeS3    SourceType = "dev.knative.eventing.s3"
)

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1alpha1", Resource: "integrationsources"}
}

// IsReady tests to see if a ContainerSource becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timing...)
}

// Install will create a ContainerSource resource, augmented with the config fn options.
func Install(name string, sourceType SourceType, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name":                  name,
		"integrationSourceType": string(sourceType),
	}
	for _, fn := range opts {
		fn(cfg)
	}

	return func(ctx context.Context, t feature.T) {
		if ic := environment.GetIstioConfig(ctx); ic.Enabled {
			manifest.WithIstioPodAnnotations(cfg)
		}

		//if err := registerImage(ctx); err != nil {
		//	t.Fatal(err)
		//}
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
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

		if d.Audience != nil {
			sink["audience"] = *d.Audience
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
