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

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	loggingtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/metrics"
	tracingconfig "knative.dev/pkg/tracing/config"

	_ "knative.dev/pkg/metrics/testing"
)

const testComponent = "test_component"

func TestNewConfigWatcher_defaults(t *testing.T) {
	ctx := loggingtesting.TestContextWithLogger(t)
	cw := WatchConfigurations(ctx, testComponent, newTestConfigMapWatcher())

	assert.NotNil(t, cw.LoggingConfig(), "logging config should not be nil")
	assert.NotNil(t, cw.MetricsConfig(), "metrics config should not be nil")
	assert.NotNil(t, cw.TracingConfig(), "tracing config should not be nil")

	envs := cw.ToEnvVars()

	const expectEnvs = 3
	require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)

	assert.Equal(t, EnvLoggingCfg, envs[0].Name, "first env var is logging config")
	assert.Contains(t, envs[0].Value, "zap-logger-config")

	assert.Equal(t, EnvMetricsCfg, envs[1].Name, "second env var is metrics config")
	assert.Contains(t, envs[1].Value, `"metrics.backend":"prometheus"`)

	assert.Equal(t, EnvTracingCfg, envs[2].Name, "third env var is tracing config")
	assert.Contains(t, envs[2].Value, `"backend":"zipkin"`)
}

func TestNewConfigWatcher_withOptions(t *testing.T) {
	ctx := loggingtesting.TestContextWithLogger(t)
	cw := WatchConfigurations(ctx, testComponent, newTestConfigMapWatcher(),
		WithMetrics,
	)

	assert.Nil(t, cw.LoggingConfig(), "logging config should be nil")
	assert.Nil(t, cw.TracingConfig(), "tracing config should be nil")
	assert.NotNil(t, cw.MetricsConfig(), "metrics config should not be nil")

	envs := cw.ToEnvVars()

	const expectEnvs = 1
	require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)

	assert.Equal(t, EnvMetricsCfg, envs[0].Name, "env var is metrics config")
	assert.Contains(t, envs[0].Value, `"metrics.backend":"prometheus"`)
}

func newTestConfigMapWatcher() configmap.Watcher {
	return configmap.NewStaticWatcher(
		newTestConfigMap(logging.ConfigMapName(), loggingConfigMapData()),
		newTestConfigMap(metrics.ConfigMapName(), metricsConfigMapData()),
		newTestConfigMap(tracingconfig.ConfigName, tracingConfigMapData()),
	)
}

func newTestConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	data["_example"] = "test-config"

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	}
}

// ConfigMap data generators
func loggingConfigMapData() map[string]string {
	return map[string]string{
		"zap-logger-config": `{"level": "info"}`,
	}
}
func metricsConfigMapData() map[string]string {
	return map[string]string{
		"metrics.backend": "prometheus",
	}
}
func tracingConfigMapData() map[string]string {
	return map[string]string{
		"backend":         "zipkin",
		"zipkin-endpoint": "zipkin.test",
	}
}
