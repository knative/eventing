package source

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	_ "knative.dev/pkg/metrics/testing"
)

func TestConfigWatcher_defaults(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	cw := StartWatchingSourceConfigurations(ctx, "name", configmap.NewStaticWatcher(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-observability",
			Namespace: "knative-eventing",
		},
		Data: map[string]string{
			"_example": "test-config",
		},
	}, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-logging",
			Namespace: "knative-eventing",
		},
		Data: map[string]string{
			"_example": "test-config",
		},
	}, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-tracing",
			Namespace: "knative-eventing",
		},
		Data: map[string]string{
			"_example": "test-config",
		},
	}))

	if cw.MetricsConfig() == nil {
		t.Error("Expecting metrics config to be non nil")
	}
	if cw.LoggingConfig() == nil {
		t.Error("Expecting logging config to be non nil")
	}
	if cw.TracingConfig() == nil {
		t.Error("Expecting tracing config to be non nil")
	}
}
