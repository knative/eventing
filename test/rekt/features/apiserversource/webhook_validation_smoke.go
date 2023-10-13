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

	"github.com/stretchr/testify/assert"
	"knative.dev/reconciler-test/pkg/resources/service"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/reconciler-test/pkg/feature"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
)

func CreateWithInvalidSpec() *feature.Feature {
	srcname := feature.MakeRandomK8sName("apiserversource")

	f := feature.NewFeatureNamed("ApiServerSource webhook is configured correctly.")

	f.Stable("ApiServerSource webhook").
		Must("reject invalid spec on resource creation", createApiServerSourceWithInvalidSpec(srcname))

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
			apiserversource.WithSink(service.AsDestinationRef("foo-svc")),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
			apiserversource.WithServiceAccountName("foo-sa"),
			// invalid event mode
			apiserversource.WithEventMode("Unknown"))

		if err != nil {
			// all good, error is expected
			assert.ErrorContains(t, err, `admission webhook "validation.webhook.eventing.knative.dev" denied the request: validation failed: invalid value: Unknown: spec.mode`)
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
			assert.ErrorContains(t, err, `admission webhook "validation.webhook.eventing.knative.dev" denied the request: validation failed: invalid value: Unknown: spec.mode`)
		} else {
			t.Error("expected ApiServerResource to reject invalid spec.")
		}
	}
}
