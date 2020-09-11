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
	"context"
	"testing"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/test/zipkin"
)

// Setup sets up port forwarding to Zipkin.
func Setup(t *testing.T, client *testlib.Client) {
	zipkin.SetupZipkinTracingFromConfigTracingOrFail(context.Background(), t, client.Kube.Kube, resources.SystemNamespace)
}
