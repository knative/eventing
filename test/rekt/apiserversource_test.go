// +build e2e

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

package rekt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/sources/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/state"
)

// TestApiServerSourceValidationWebhookConfigurationOnCreate tests if the webhook
// is configured correctly for ApiServerSource validation on resource creation.
func TestApiServerSourceValidationWebhookConfigurationOnCreate(t *testing.T) {
	t.Parallel()

	srcname := feature.MakeRandomK8sName("apiserversource")

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	f := feature.NewFeatureNamed("ApiServerSource webhook is configured correctly.")

	f.Stable("ApiServerSource webhook").
		Must("reject invalid spec on resource creation", createApiServerSourceWithInvalidSpec(srcname))

	env.Test(ctx, t, f)
}

// TestApiServerSourceValidationWebhookConfigurationOnUpdate tests if the webhook
// is configured correctly for ApiServerSource validation on resource update.
func TestApiServerSourceValidationWebhookConfigurationOnUpdate(t *testing.T) {
	t.Parallel()

	srcname := feature.MakeRandomK8sName("apiserversource")

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	f := feature.NewFeatureNamed("ApiServerSource webhook is configured correctly.")

	f.Setup("Set ApiServerSource Name", setApiServerSourceName(srcname))

	f.Setup("Create valid ApiServerSource", createApiServerSourceWithValidSpec(srcname))

	f.Stable("ApiServerSource webhook").
		Must("reject invalid spec", updateApiServerSourceWithInvalidSpec())

	env.Test(ctx, t, f)
}

func createApiServerSourceWithValidSpec(name string) func(ctx context.Context, t feature.T) {
	return func(ctx context.Context, t feature.T) {
		_, err := apiserversource.InstallLocalYaml(ctx, name, apiServerSourceWithValidSpec())

		// we don't care if the resource gets ready or not as we only concerned about the webhook

		if err != nil {
			t.Error("ApiServerResource with valid spec cannot be applied", err)
		}
	}
}

func createApiServerSourceWithInvalidSpec(name string) func(ctx context.Context, t feature.T) {
	return func(ctx context.Context, t feature.T) {
		// generate a valid spec, then change the event mode with an invalid value, then
		// try actually applying the resource
		_, err := apiserversource.InstallLocalYaml(ctx, name,
			apiServerSourceWithValidSpec(), apiserversource.WithEventMode("Unknown"))

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

func apiServerSourceWithValidSpec() manifest.CfgFn {
	withSink := apiserversource.WithSink(&duckv1.KReference{
		Kind:       "Service",
		Name:       "foo-svc",
		APIVersion: "v1",
	}, "")

	withResources := apiserversource.WithResources(v1.APIVersionKindSelector{
		APIVersion: "v1",
		Kind:       "Event",
	})

	withServiceAccountName := apiserversource.WithServiceAccountName("foo-sa")
	withEventMode := apiserversource.WithEventMode(v1.ReferenceMode)

	return func(cfg map[string]interface{}) {
		withSink(cfg)
		withResources(cfg)
		withServiceAccountName(cfg)
		withEventMode(cfg)
	}
}

type SourcesClient struct {
	ApiServerSources sourcesclientsetv1.ApiServerSourceInterface
}

func Client(ctx context.Context) *SourcesClient {
	sc := eventingclient.Get(ctx).SourcesV1()
	env := environment.FromContext(ctx)

	return &SourcesClient{
		ApiServerSources: sc.ApiServerSources(env.Namespace()),
	}
}

const (
	ApiServerSourceNameKey = "apiServerSourceName"
)

func getApiServerSource(ctx context.Context, t feature.T) *sourcesv1.ApiServerSource {
	c := Client(ctx)
	name := state.GetStringOrFail(ctx, t, ApiServerSourceNameKey)

	src, err := c.ApiServerSources.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get ApiServerSource, %v", err)
	}
	return src
}

func setApiServerSourceName(name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, ApiServerSourceNameKey, name)
	}
}
