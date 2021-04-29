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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/sources/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

func ControlPlaneConformance(apiServerSourceName string) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource Specification - Control Plane",
		Features: []feature.Feature{
			*ControlPlaneApiServerSource(apiServerSourceName),
		},
	}

	return fs
}

func ControlPlaneApiServerSource(apiServerSourceName string) *feature.Feature {
	f := feature.NewFeatureNamed("Conformance")

	f.Setup("Set ApiServerSource Name", setApiServerSourceName(apiServerSourceName))

	f.Stable("EventMode").
		Must("ApiServerSource MUST allow retrieving the event payloads of ObjectReference or ResourceEvent",
			eventMode()).
		Must("ApiServerSource MUST reject disallowed eventModes",
			disallowedEventMode())

	return f
}

func eventMode() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for _, mode := range []string{"Reference", "Resource"} {
			apiServerSource := getApiServerSource(ctx, t)

			apiServerSource.Spec.EventMode = mode

			_, err := Client(ctx).ApiServerSources.Update(ctx, apiServerSource, metav1.UpdateOptions{})
			if err != nil {
				t.Errorf("expected ApiServerResource to accept the allowed eventMode %q. Err: %e", mode, err.Error())
			}
		}
	}
}

func disallowedEventMode() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for _, mode := range []string{"Disallowed"} {
			apiServerSource := getApiServerSource(ctx, t)

			apiServerSource.Spec.EventMode = mode

			_, err := Client(ctx).ApiServerSources.Update(ctx, apiServerSource, metav1.UpdateOptions{})
			if err != nil {
				// all good, error is expected
			} else {
				t.Errorf("expected ApiServerResource to reject the disallowed eventMode %q.", mode)
			}
		}
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
