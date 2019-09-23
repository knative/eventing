/*
Copyright 2019 The Knative Authors

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

package tracing

import (
	"sync"
	"testing"

	"knative.dev/eventing/test/common"
	"knative.dev/pkg/test/zipkin"
)

// Setup sets up port forwarding to Zipkin and sets the knative-eventing tracing config to debug
// mode (everything is sampled).
func Setup(t *testing.T, client *common.Client) {
	// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
	// TestMain.
	zipkin.SetupZipkinTracing(client.Kube.Kube, t.Logf)
	setTracingConfigToDebugMode(t, client)
}

var setTracingConfigOnce = sync.Once{}

// TODO Do we need a tear down method as well?
func setTracingConfigToDebugMode(t *testing.T, client *common.Client) {
	setTracingConfigOnce.Do(func() {
		err := client.Kube.UpdateConfigMap("knative-eventing", "config-tracing", map[string]string{
			"backend":         "zipkin",
			"zipkin-endpoint": "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
			"debug":           "true",
			"sample-rate":     "1.0",
		})
		if err != nil {
			t.Fatalf("Unable to set the ConfigMap: %v", err)
		}
	})
}
