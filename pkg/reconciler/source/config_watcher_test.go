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

	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
)

const testComponent = "test_component"
const testComponentWithCustomLogLevel = "test_component_custom_loglevel"

func TestNewConfigWatcher_defaults(t *testing.T) {
	testCases := []struct {
		name                        string
		cmw                         configmap.Watcher
		expectLoggingContains       string
		expectObservabilityContains string
	}{{
		name:                  "With pre-filled sample data",
		cmw:                   configMapWatcherWithSampleData(),
		expectLoggingContains: `"zap-logger-config":"{\"level\": \"fatal\"}"`,
		expectObservabilityContains: `"metrics":{"protocol":"http/protobuf","endpoint":"http://localhost:12345"}`,
	}, {
		name: "With empty data",
		cmw:  configMapWatcherWithEmptyData(),
		// logging defaults to Knative's defaults
		expectLoggingContains: ``,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := loggingtesting.TestContextWithLogger(t)
			cw := WatchConfigurations(ctx, testComponent, tc.cmw)

			assert.NotNil(t, cw.LoggingConfig(), "logging config should be enabled")

			envs := cw.ToEnvVars()

			const expectEnvs = 2
			require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)

			assert.Equal(t, EnvLoggingCfg, envs[0].Name, "first env var is logging config")
			assert.Contains(t, envs[0].Value, tc.expectLoggingContains)
			assert.Contains(t, envs[1].Value, tc.expectObservabilityContains)
		})
	}
}

func TestLoggingConfigWithCustomLoggingLevel(t *testing.T) {
	ctx := loggingtesting.TestContextWithLogger(t)
	cw := WatchConfigurations(ctx, testComponentWithCustomLogLevel, configMapWatcherWithSampleData())

	envs := cw.ToEnvVars()

	require.Equal(t, EnvLoggingCfg, envs[0].Name, "first env var is logging config")

	const expectLoggingContains = `"zap-logger-config":"{\"level\":\"debug\",`
	assert.Contains(t, envs[0].Value, expectLoggingContains)
}

func TestEmptyVarsGenerator(t *testing.T) {
	g := &EmptyVarsGenerator{}
	envs := g.ToEnvVars()
	const expectEnvs = 2
	require.Lenf(t, envs, expectEnvs, "there should be %d env var(s)", expectEnvs)
}

func TestNilSafeMethods(t *testing.T) {
	var cw *ConfigWatcher
	assert.Nil(t, cw.LoggingConfig(), "logging config should be disabled")
	assert.Nil(t, cw.ObservabilityConfig(), "tracing config should be disabled")

	assert.NotPanics(t, func() {
		cw.updateFromLoggingConfigMap(nil)
		cw.updateFromObservabilityConfigMap(nil)
	}, "can update nil cfg")

}

// configMapWatcherWithSampleData constructs a Watcher for static sample data.
func configMapWatcherWithSampleData() configmap.Watcher {
	return configmap.NewStaticWatcher(
		newTestConfigMap(logging.ConfigMapName(), loggingConfigMapData()),
		newTestConfigMap(o11yconfigmap.Name(), observabilityConfigMapData()),
	)
}

// configMapWatcherWithEmptyData constructs a Watcher for empty data.
func configMapWatcherWithEmptyData() configmap.Watcher {
	return configmap.NewStaticWatcher(
		newTestConfigMap(logging.ConfigMapName(), nil),
		newTestConfigMap(o11yconfigmap.Name(), nil),
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

		"loglevel." + testComponentWithCustomLogLevel: "debug",
	}
}
func observabilityConfigMapData() map[string]string {
	return map[string]string{
		"metrics-protocol": "http/protobuf",
		"metrics-endpoint": "http://localhost:12345",
		"tracing-protocol": "grpc",
		"tracing-endpoint": "http://localhost:54321",
	}
}
