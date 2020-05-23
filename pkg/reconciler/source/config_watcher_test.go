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
	testCases := []struct {
		name                  string
		cmw                   configmap.Watcher
		expectLoggingContains string
		expectMetricsContains string
		expectTracingContains string
	}{
		{
			name:                  "With pre-filled sample data",
			cmw:                   configMapWatcherWithSampleData(),
			expectLoggingContains: `"zap-logger-config":"{\"level\": \"fatal\"}"`,
			expectMetricsContains: `"ConfigMap":{"metrics.backend":"test"}`,
			expectTracingContains: `"zipkin-endpoint":"zipkin.test"`,
		},
		{
			name: "With empty data",
			cmw:  configMapWatcherWithEmptyData(),
			// logging defaults to Knative's defaults
			expectLoggingContains: `{"zap-logger-config":"{\n  \"level\": \"info\"`,
			// metrics defaults to empty ConfigMap
			expectMetricsContains: `"ConfigMap":{}`,
			// tracing defaults to None backend
			expectTracingContains: `"backend":"none"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := loggingtesting.TestContextWithLogger(t)
			cw := WatchConfigurations(ctx, testComponent, tc.cmw)

			assert.NotNil(t, cw.LoggingConfig(), "logging config should be enabled")
			assert.NotNil(t, cw.MetricsConfig(), "metrics config should be enabled")
			assert.NotNil(t, cw.TracingConfig(), "tracing config should be enabled")

			envs := cw.ToEnvVars()

			const expectEnvs = 3
			require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)

			assert.Equal(t, EnvLoggingCfg, envs[0].Name, "first env var is logging config")
			assert.Contains(t, envs[0].Value, tc.expectLoggingContains)

			assert.Equal(t, EnvMetricsCfg, envs[1].Name, "second env var is metrics config")
			assert.Contains(t, envs[1].Value, tc.expectMetricsContains)

			assert.Equal(t, EnvTracingCfg, envs[2].Name, "third env var is tracing config")
			assert.Contains(t, envs[2].Value, tc.expectTracingContains)
		})
	}
}

func TestNewConfigWatcher_withOptions(t *testing.T) {
	testCases := []struct {
		name                  string
		cmw                   configmap.Watcher
		expectMetricsContains string
	}{
		{
			name:                  "With pre-filled sample data",
			cmw:                   configMapWatcherWithSampleData(),
			expectMetricsContains: `"ConfigMap":{"metrics.backend":"test"}`,
		},
		{
			name: "With empty data",
			cmw:  configMapWatcherWithEmptyData(),
			// metrics defaults to empty ConfigMap
			expectMetricsContains: `"ConfigMap":{}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := loggingtesting.TestContextWithLogger(t)
			cw := WatchConfigurations(ctx, testComponent, tc.cmw,
				WithMetrics,
			)

			assert.Nil(t, cw.LoggingConfig(), "logging config should be disabled")
			assert.Nil(t, cw.TracingConfig(), "tracing config should be disabled")
			assert.NotNil(t, cw.MetricsConfig(), "metrics config should be enabled")

			envs := cw.ToEnvVars()

			const expectEnvs = 1
			require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)

			assert.Equal(t, EnvMetricsCfg, envs[0].Name, "env var is metrics config")
			assert.Contains(t, envs[0].Value, tc.expectMetricsContains)
		})
	}
}

// configMapWatcherWithSampleData constructs a Watcher for static sample data.
func configMapWatcherWithSampleData() configmap.Watcher {
	return configmap.NewStaticWatcher(
		newTestConfigMap(logging.ConfigMapName(), loggingConfigMapData()),
		newTestConfigMap(metrics.ConfigMapName(), metricsConfigMapData()),
		newTestConfigMap(tracingconfig.ConfigName, tracingConfigMapData()),
	)
}

// configMapWatcherWithEmptyData constructs a Watcher for empty data.
func configMapWatcherWithEmptyData() configmap.Watcher {
	return configmap.NewStaticWatcher(
		newTestConfigMap(logging.ConfigMapName(), nil),
		newTestConfigMap(metrics.ConfigMapName(), nil),
		newTestConfigMap(tracingconfig.ConfigName, nil),
	)
}

func newTestConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	if data == nil {
		data = make(map[string]string, 1)
	}

	// _example key is always appended to mimic Knative's release manifests
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
		"zap-logger-config": `{"level": "fatal"}`,
	}
}
func metricsConfigMapData() map[string]string {
	return map[string]string{
		"metrics.backend": "test",
	}
}
func tracingConfigMapData() map[string]string {
	return map[string]string{
		"backend":         "zipkin",
		"zipkin-endpoint": "zipkin.test",
	}
}
