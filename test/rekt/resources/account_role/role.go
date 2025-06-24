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

//nolint:staticcheck  // ST1003: underscore in package name
package account_role

import (
	"context"
	"embed"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

// Install will create a channelable-manipulator bound service account,
// augmented with the config fn options.
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

func WithRole(role string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["role"] = role
	}
}

func WithRules(rules ...rbacv1.PolicyRule) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["rules"]; !set {
			cfg["rules"] = []map[string]interface{}{}
		}

		for _, resource := range rules {
			elem := map[string]interface{}{
				"apiGroups": resource.APIGroups,
				"resources": resource.Resources,
				"verbs":     resource.Verbs,
				// TODO: skip for now
				//"resourceNames":   resource.ResourceNames,
				//"nonResourceURLs": resource.NonResourceURLs,
			}
			cfg["rules"] = append(cfg["rules"].([]map[string]interface{}), elem)
		}
	}
}

func WithRoleMatchLabel(matchLabel string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["matchLabel"] = matchLabel
	}
}

func AsChannelableManipulator(cfg map[string]interface{}) {
	WithRole(fmt.Sprintf("channelable-manipulator-collector-%s", cfg["name"]))(cfg)
	WithRoleMatchLabel("duck.knative.dev/channelable")(cfg)
}

func AsAddressableResolver(cfg map[string]interface{}) {
	WithRole(fmt.Sprintf("addressable-resolver-collector-%s", cfg["name"]))(cfg)
	WithRoleMatchLabel("duck.knative.dev/addressable")(cfg)
}
