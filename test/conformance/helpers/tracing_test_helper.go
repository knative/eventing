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

package helpers

import (
	"context"
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
)

// SetupTracingTestInfrastructureFunc sets up the infrastructure for running tracing tests. It returns the
// expected trace as well as a string that is expected to be in the logger Pod's logs.
type SetupTracingTestInfrastructureFunc func(
	ctx context.Context,
	t *testing.T,
	channel *metav1.TypeMeta,
	client *testlib.Client,
	loggerPodName string,
	senderPublishTrace bool,
) (tracinghelper.TestSpanTree, cetest.EventMatcher)

// tracingTest bootstraps the test and then executes the assertions on the received event and on the spans
func tracingTest(
	ctx context.Context,
	t *testing.T,
	setupClient testlib.SetupClientOption,
	setupInfrastructure SetupTracingTestInfrastructureFunc,
	channel metav1.TypeMeta,
) {
	client := testlib.Setup(t, true, setupClient)
	defer testlib.TearDown(client)

	// TODO - redo with OTel - https://github.com/knative/eventing/issues/8853
}
