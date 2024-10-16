package integrationsource

import (
	"context"
	"embed"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"strings"
	"time"
)

//go:embed integrationsource.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1alpha1", Resource: "integrationsources"}
}

// IsReady tests to see if a ContainerSource becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timing...)
}

// Install will create a ContainerSource resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
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
