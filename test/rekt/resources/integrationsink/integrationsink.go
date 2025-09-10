package integrationsink

import (
	"context"
	"embed"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed integrationsink.yaml
var yamlEmbed embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sinks.knative.dev", Version: "v1alpha1", Resource: "integrationsinks"}
}

// WithAnnotations adds annotations to the IntegrationSink.
func WithAnnotations(annotations map[string]any) manifest.CfgFn {
	return func(cfg map[string]any) {
		if annotations != nil {
			cfg["annotations"] = annotations
		}
	}
}

// Install will create a resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {

	return func(ctx context.Context, t feature.T) {
		cfg := map[string]any{
			"name":                           name,
			"namespace":                      environment.FromContext(ctx).Namespace(),
			"image":                          eventshub.ImageFromContext(ctx),
			eventshub.ConfigLoggingEnv:       knative.LoggingConfigFromContext(ctx),
			eventshub.ConfigObservabilityEnv: knative.ObservabilityConfigFromContext(ctx),
		}
		for _, fn := range opts {
			fn(cfg)
		}

		if _, err := manifest.InstallYamlFS(ctx, yamlEmbed, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a IntegrationSink becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsNotReady tests to see if a IntegrationSink becomes NotReady within the time given.
func IsNotReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsNotReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a IntegrationSink becomes addressable within the  time
// given.
func IsAddressable(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timings...)
}

func GoesReadySimple(name string) *feature.Feature {
	f := feature.NewFeature()

	f.Setup("install integration sink", Install(name))
	f.Setup("integrationsink is ready", IsReady(name))
	f.Setup("integrationsink is addressable", IsAddressable(name))

	return f
}
