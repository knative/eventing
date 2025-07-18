/*
Copyright 2025 The Knative Authors

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

package otel

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stoolmetrics "k8s.io/client-go/tools/metrics"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/observability/configmap"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	o11yconfigmap "knative.dev/pkg/observability/configmap"
	"knative.dev/pkg/observability/metrics"
	k8smetrics "knative.dev/pkg/observability/metrics/k8s"
	"knative.dev/pkg/observability/resource"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/observability/tracing"
	"knative.dev/pkg/system"
)

func SetupObservabilityOrDie(
	ctx context.Context,
	component string,
	logger *zap.SugaredLogger,
	pprof *k8sruntime.ProfilingServer,
) (*metrics.MeterProvider, *tracing.TracerProvider) {
	cfg, err := GetObservabilityConfig(ctx)
	if err != nil {
		logger.Fatalw("error loading observability configuration", zap.Error(err))
	}

	pprof.UpdateFromConfig(cfg.Runtime)

	otelResource := resource.Default(component)

	meterProvider, err := metrics.NewMeterProvider(
		ctx,
		cfg.Metrics,
		metric.WithResource(otelResource),
	)
	if err != nil {
		logger.Fatalw("failed to set up meter provider", zap.Error(err))
	}

	otel.SetMeterProvider(meterProvider)

	workQueueMetrics, err := k8smetrics.NewWorkqueueMetricsProvider(
		k8smetrics.WithMeterProvider(meterProvider),
	)
	if err != nil {
		logger.Fatalw("failed to setup k8s workqueue metrics", zap.Error(err))
	}

	workqueue.SetProvider(workQueueMetrics)
	controller.SetMetricsProvider(workQueueMetrics)

	clientMetrics, err := k8smetrics.NewClientMetricProvider(
		k8smetrics.WithMeterProvider(meterProvider),
	)
	if err != nil {
		logger.Fatalw("failed to setup k8s client-go metrics", zap.Error(err))
	}

	k8stoolmetrics.Register(k8stoolmetrics.RegisterOpts{
		RequestLatency: clientMetrics.RequestLatencyMetric(),
		RequestResult:  clientMetrics.RequestResultMetric(),
	})

	err = runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(cfg.Runtime.ExportInterval),
	)
	if err != nil {
		logger.Fatalw("failed to start runtime metrics", zap.Error(err))
	}

	tracerProvider, err := tracing.NewTracerProvider(
		ctx,
		cfg.Tracing,
		trace.WithResource(otelResource),
	)
	if err != nil {
		logger.Fatalw("failed to setup trace provider", zap.Error(err))
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)

	return meterProvider, tracerProvider
}

func DefaultMeterProvider(ctx context.Context, resource *sdkresource.Resource) *metrics.MeterProvider {
	meterProvider, _ := metrics.NewMeterProvider(
		ctx,
		observability.DefaultConfig().Metrics,
		metric.WithResource(resource),
	)

	return meterProvider
}

func DefaultTraceProvider(ctx context.Context, resource *sdkresource.Resource) *tracing.TracerProvider {
	traceProvider, _ := tracing.NewTracerProvider(
		ctx,
		observability.DefaultConfig().Tracing,
		trace.WithResource(resource),
	)

	return traceProvider
}

// GetObservabilityConfig gets the observability config from the (in order):
// 1. the provided context,
// 2. from the API server,
// 3. default values (if not found and there were no other errors in the api server request).
func GetObservabilityConfig(ctx context.Context) (*observability.Config, error) {
	if cfg := observability.GetConfig(ctx); cfg != nil {
		return cfg, nil
	}

	cm, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).
		Get(ctx, o11yconfigmap.Name(), metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		return observability.DefaultConfig(), nil
	} else if err != nil {
		return nil, err
	}

	return configmap.Parse(cm)
}
