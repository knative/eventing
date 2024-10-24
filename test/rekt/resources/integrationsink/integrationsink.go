package integrationsink

import (
	"context"
	"embed"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"time"
)

//go:embed integrationsink.yaml
var yamlEmbed embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sinks.knative.dev", Version: "v1alpha1", Resource: "integrationsinks"}
}

// WithAnnotations adds annotations to the IntegrationSink.
func WithAnnotations(annotations map[string]interface{}) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if annotations != nil {
			cfg["annotations"] = annotations
		}
	}
}

// Install will create a resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {

	return func(ctx context.Context, t feature.T) {
		cfg := map[string]interface{}{
			"name":                     name,
			"namespace":                environment.FromContext(ctx).Namespace(),
			"image":                    eventshub.ImageFromContext(ctx),
			eventshub.ConfigLoggingEnv: knative.LoggingConfigFromContext(ctx),
			eventshub.ConfigTracingEnv: knative.TracingConfigFromContext(ctx),
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
