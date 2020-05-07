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
	"time"

	"knative.dev/pkg/test/zipkin"

	"knative.dev/eventing/test/lib"

	pkgtesting "knative.dev/pkg/configmap/testing"
)

// Setup sets up port forwarding to Zipkin and sets the knative-eventing tracing config to debug
// mode (everything is sampled).
func Setup(t *testing.T, client *lib.Client) {
	// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
	// TestMain.
	if !zipkin.SetupZipkinTracing(client.Kube.Kube, t.Logf) {
		t.Fatalf("Unable to set up Zipkin for tracking")
	}
	setTracingConfigToZipkin(t, client)
}

var setTracingConfigOnce = sync.Once{}

// setTracingConfigToZipkin sets the tracing configuration to point at the standard Zipkin endpoint
// installed by the e2e test setup scripts.
// Note that this used to set the sampling rate to 100%. We _think_ that overwhelmed the Zipkin
// instance and caused https://github.com/knative/eventing/issues/2040. So now we just ensure that
// the tests that test tracing ensure that the requests are made with the sampled flag set to true.
// TODO Do we need a tear down method to revert the config map to its original state?
func setTracingConfigToZipkin(t *testing.T, client *lib.Client) {
	setTracingConfigOnce.Do(func() {
		tracingConfig, exampleTracingConfig := pkgtesting.ConfigMapsFromTestFile(t, "config-tracing")
		_, backendOk := tracingConfig.Data["backend"]
		_, endpointOk := tracingConfig.Data["endpoint"]
		var err error
		if backendOk && endpointOk {
			_, err = client.Kube.GetConfigMap(tracingConfig.GetNamespace()).Update(tracingConfig)
		} else {
			_, err = client.Kube.GetConfigMap(exampleTracingConfig.GetNamespace()).Update(exampleTracingConfig)
		}
		if err != nil {
			t.Fatalf("Unable to set the ConfigMap: %v", err)
		}
		// Wait for 5 seconds to let the ConfigMap be synced up.
		time.Sleep(5 * time.Second)
	})
}
