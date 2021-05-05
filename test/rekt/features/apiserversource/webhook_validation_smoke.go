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

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"

	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

func CreateWithInvalidSpec() *feature.Feature {
	srcname := feature.MakeRandomK8sName("apiserversource")

	f := feature.NewFeatureNamed("ApiServerSource webhook is configured correctly.")

	f.Stable("ApiServerSource webhook").
		Must("reject invalid spec on resource creation", createApiServerSourceWithInvalidSpec(srcname))

	return f
}

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

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

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.add")).AtLeast(1))

	return f
}

func UpdateWithInvalidSpec(name string) *feature.Feature {
	f := feature.NewFeatureNamed("ApiServerSource webhook is configured correctly.")

	f.Setup("Set ApiServerSource Name", setApiServerSourceName(name))

	f.Stable("ApiServerSource webhook").
		Must("reject invalid spec", updateApiServerSourceWithInvalidSpec())

	return f
}

func createApiServerSourceWithInvalidSpec(name string) func(ctx context.Context, t feature.T) {
	return func(ctx context.Context, t feature.T) {
		_, err := apiserversource.InstallLocalYaml(ctx, name,
			apiserversource.WithSink(&duckv1.KReference{
				Kind:       "Service",
				Name:       "foo-svc",
				APIVersion: "v1",
			}, ""),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
			apiserversource.WithServiceAccountName("foo-sa"),
			// invalid event mode
			apiserversource.WithEventMode("Unknown"))

		if err != nil {
			// all good, error is expected
			assert.EqualError(t, err, `admission webhook "validation.webhook.eventing.knative.dev" denied the request: validation failed: invalid value: Unknown: spec.mode`)
		} else {
			t.Error("expected ApiServerResource to reject invalid spec.")
		}
	}
}

func updateApiServerSourceWithInvalidSpec() func(ctx context.Context, t feature.T) {
	return func(ctx context.Context, t feature.T) {
		apiServerSource := getApiServerSource(ctx, t)

		apiServerSource.Spec.EventMode = "Unknown"

		_, err := Client(ctx).ApiServerSources.Update(ctx, apiServerSource, metav1.UpdateOptions{})

		if err != nil {
			// all good, error is expected
			assert.EqualError(t, err, `admission webhook "validation.webhook.eventing.knative.dev" denied the request: validation failed: invalid value: Unknown: spec.mode`)
		} else {
			t.Error("expected ApiServerResource to reject invalid spec.")
		}
	}
}
