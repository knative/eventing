/*
Copyright 2020 The Knative Authors

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

package adapter

import (
	"context"
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/reconciler"
	_ "knative.dev/pkg/system/testing"
)

const (
	tNamespace                  = "test"
	tLoggerConfigMapName        = "logger-config-test"
	tObservabilityConfigMapName = "observability-config-test"
	tTracingConfigMapName       = "tracing-config-test"
	tComponentName              = "test-component"
	tMetricsDomain              = "test-metrics-domain"
)

type myAdapter struct {
	blocking bool
}

func TestMainWithContext(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", "error config")
	os.Setenv("K_LOGGING_CONFIG", "error config")
	os.Setenv("MODE", "mymode")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_METRICS_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
	}()

	ctx := context.TODO()
	ctx, _ = fakekubeclient.With(ctx)

	MainWithContext(ctx, "mycomponent",
		func() EnvConfigAccessor { return &myEnvConfig{} },
		func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
			env := processed.(*myEnvConfig)

			if env.Mode != "mymode" {
				t.Error("Expected mode mymode, got:", env.Mode)
			}

			if env.Sink != "http://sink" {
				t.Error("Expected sinkURI http://sink, got:", env.Sink)
			}

			if leaderelection.HasLeaderElection(ctx) {
				t.Error("Expected no leader election, but got leader election")
			}
			return &myAdapter{}
		})

	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}

func TestMainWithInformerNoLeaderElection(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", "error config")
	os.Setenv("K_LOGGING_CONFIG", "error config")
	os.Setenv("MODE", "mymode")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_METRICS_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
	}()

	ctx, cancel := context.WithCancel(context.TODO())
	ctx, _ = fakekubeclient.With(ctx)
	env := ConstructEnvOrDie(func() EnvConfigAccessor { return &myEnvConfig{} })
	done := make(chan bool)
	go func() {
		MainWithInformers(ctx,
			"mycomponent",
			env,
			func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
				env := processed.(*myEnvConfig)

				if env.Mode != "mymode" {
					t.Error("Expected mode mymode, got:", env.Mode)
				}

				if env.Sink != "http://sink" {
					t.Error("Expected sinkURI http://sink, got:", env.Sink)
				}

				if leaderelection.HasLeaderElection(ctx) {
					t.Error("Expected no leader election, but got leader election")
				}
				return &myAdapter{blocking: true}
			})
		done <- true
	}()

	cancel()
	<-done
	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}

func TestMain_MetricsConfig(t *testing.T) {
	m := &metrics.ExporterOptions{
		Domain:         "example.com",
		Component:      "foo",
		PrometheusPort: 9021,
		PrometheusHost: "prom.example.com",
		ConfigMap: map[string]string{
			"profiling.enable": "true",
			"foo":              "bar",
		},
	}
	metricsJson, _ := metrics.OptionsToJSON(m)

	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", metricsJson)
	os.Setenv("K_LOGGING_CONFIG", "error config")
	os.Setenv("MODE", "mymode")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_METRICS_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
	}()

	ctx, cancel := context.WithCancel(context.TODO())
	ctx, _ = fakekubeclient.With(ctx)
	env := ConstructEnvOrDie(func() EnvConfigAccessor { return &myEnvConfig{} })
	done := make(chan bool)
	go func() {
		MainWithInformers(ctx,
			"mycomponent",
			env,
			func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
				env := processed.(*myEnvConfig)

				if env.Mode != "mymode" {
					t.Error("Expected mode mymode, got:", env.Mode)
				}

				if env.Sink != "http://sink" {
					t.Error("Expected sinkURI http://sink, got:", env.Sink)
				}

				if leaderelection.HasLeaderElection(ctx) {
					t.Error("Expected no leader election, but got leader election")
				}
				return &myAdapter{blocking: true}
			})
		done <- true
	}()

	cancel()
	<-done
	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}

func TestStartInformers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	StartInformers(ctx, nil)
	cancel()
}

func TestWithInjectorEnabled(t *testing.T) {
	_ = WithInjectorEnabled(context.TODO())
}

func TestConstructEnvOrDie(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected to die")
			}
			cancel()
		}()
		ConstructEnvOrDie(func() EnvConfigAccessor {
			return nil
		})
	}()
	<-ctx.Done()
}

func (m *myAdapter) Reconcile(ctx context.Context, key string) error {
	return nil
}

func (m *myAdapter) Start(ctx context.Context) error {
	if m.blocking {
		<-ctx.Done()
	}
	return nil
}

func (m *myAdapter) Promote(b reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
	return nil
}

func (m *myAdapter) Demote(reconciler.Bucket) {}

func TestMain_LogConfigWatcher(t *testing.T) {
	lcm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tNamespace,
			Name:      tLoggerConfigMapName,
		},
		Data: map[string]string{
			"zap-logger-config":          `{"level": "info"}`,
			"loglevel." + tComponentName: "info",
		},
	}
	ocm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tNamespace,
			Name:      tObservabilityConfigMapName,
		},
		Data: map[string]string{
			"metrics.backend-destination":       "prometheus",
			"profiling.enable":                  "false",
			"sink-event-error-reporting.enable": "true",
		},
	}
	tcm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tNamespace,
			Name:      tTracingConfigMapName,
		},
		Data: map[string]string{
			"backend": "none",
		},
	}

	os.Setenv("K_SINK", "http://sink")
	os.Setenv("K_LOGGING_CONFIG", "this should not be applied")
	os.Setenv("NAMESPACE", "ns from context should be used")
	os.Setenv("POD_NAME", "my-test-pod")
	os.Setenv(metrics.DomainEnv, "env-metrics-domain")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("POD_NAME")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = WithNamespace(ctx, tNamespace)
	ctx, _ = kubeclient.With(ctx, lcm, ocm)

	cmw := &configmap.ManualWatcher{
		Namespace: NamespaceFromContext(ctx),
	}
	ctx = WithConfigWatcher(ctx, cmw)

	ctx = WithConfiguratorOptions(ctx, []ConfiguratorOption{
		WithLoggerConfigurator(NewLoggerConfiguratorFromConfigMap(tComponentName,
			WithLoggerConfiguratorConfigMapName(tLoggerConfigMapName))),

		// Other configurator options are also enabled to make sure that at least
		// they do not panic when ConfigMaps are updated.
		WithMetricsExporterConfigurator(
			NewMetricsExporterConfiguratorFromConfigMap(tComponentName,
				WithMetricsExporterConfiguratorConfigMapName(tObservabilityConfigMapName),
				WithMetricsExporterConfiguratorMetricsDomain(tMetricsDomain))),
		WithTracingConfigurator(NewTracingConfiguratorFromConfigMap(
			WithTracingConfiguratorConfigMapName(tTracingConfigMapName))),
		WithCloudEventsStatusReporterConfigurator(NewCloudEventsReporterConfiguratorFromConfigMap(
			WithCloudEventsStatusReporterConfiguratorConfigMapName(tObservabilityConfigMapName))),
		WithProfilerConfigurator(NewProfilerConfiguratorFromConfigMap(
			WithProfilerConfiguratorConfigMapName(tObservabilityConfigMapName))),
	})

	env := ConstructEnvOrDie(func() EnvConfigAccessor { return &myEnvConfig{} })
	done := make(chan bool)
	go func() {
		MainWithInformers(ctx, tComponentName, env,
			func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
				env := processed.(*myEnvConfig)

				if env.Sink != "http://sink" {
					t.Error("Expected sinkURI http://sink, got:", env.Sink)
				}

				logger := logging.FromContext(ctx).Desugar()

				// Check that we cannot log a debug message when info level
				// is configured.
				if r := logger.Check(zap.DebugLevel, "debug message"); r != nil {
					t.Error("Debug message should not be logged when info level is configured")
				}

				// Change the log level to debug at the ConfigMap
				lcm.Data["loglevel."+tComponentName] = "debug"
				cmw.OnChange(lcm)

				// Check that we can log the same message when debug level
				// is configured.
				if r := logger.Check(zap.DebugLevel, "debug message"); r == nil {
					t.Error("Debug message should not be logged when debug level is configured")
				}

				// Change observability ConfigMap settings.
				ocm.Data["metrics.backend-destination"] = "opencensus"
				ocm.Data["profiling.enable"] = "true"
				ocm.Data["sink-event-error-reporting.enable"] = "true"
				cmw.OnChange(ocm)

				// Change tracing ConfigMap settings.
				tcm.Data["backend"] = "zipkin"
				tcm.Data["zipkin-endpoint"] = "http://zipking-test-endpoint"
				cmw.OnChange(tcm)

				return &myAdapter{blocking: true}
			})
		done <- true
	}()

	cancel()
	<-done
	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}
