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
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

// GoesReady returns a feature testing if an ApiServerSource becomes ready.
func GoesReady(name string) *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource goes ready.")

	f.Setup("wait until ApiServerSource is ready", apiserversource.IsReady(name))

	f.Stable("ApiServerSource")

	return f
}

// Install returns a feature creating an ApiServerSource
func Install(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource is installed.")

	sink := feature.MakeRandomK8sName("sink")
	f.Setup("install a service", svc.Install(sink, "app", "rekt"))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource",
		account_role.Install(sacmName,
			account_role.WithRole(sacmName+"-clusterrole"),
			account_role.WithRules(rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			}),
		))

	cfg = append(cfg,
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(&duckv1.KReference{
			Kind:       "Service",
			Name:       sink,
			APIVersion: "v1",
		}, ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	)

	f.Setup("install an ApiServerSource", apiserversource.Install(name, cfg...))

	f.Stable("ApiServerSource")

	return f
}
