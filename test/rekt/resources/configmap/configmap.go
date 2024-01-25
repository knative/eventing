package configmap

import (
	"context"
	"embed"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cfgFeat "knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "containersources"}
}

// IsReady tests to see if a ContainerSource becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timing...)
}

// Install will create a configmap resource, augmented with the config fn options.
func Install(ctxParam context.Context, ns string, name string, opts ...manifest.CfgFn) feature.StepFn {
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

		configM, _ := client.Get(ctx).CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})

		flags, err := cfgFeat.NewFlagsConfigFromConfigMap(configM)
		if err != nil {
			t.Fatal(err)
		}

		ctx = cfgFeat.ToContext(ctx, flags)

		ctxParam = ctx
	}
}
