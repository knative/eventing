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

package configmap

import (
	"context"
	"embed"
	"strings"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

// Install will create a configmap resource and will load the configmap to the context.
func Install(name string, ns string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name":      name,
		"namespace": ns,
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

var WithLabels = manifest.WithLabels

func WithData(key, value string) manifest.CfgFn {
	return func(m map[string]interface{}) {
		if _, ok := m["data"]; !ok {
			m["data"] = map[string]string{}
		}
		value = strings.ReplaceAll(value, "\n", "\n    ")
		m["data"].(map[string]string)[key] = value
	}
}
