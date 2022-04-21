//go:build e2e
// +build e2e

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

package conformance

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/test/conformance/helpers"
	testlib "knative.dev/eventing/test/lib"
)

func TestBrokerV1ControlPlane(t *testing.T) {
	brokerTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {

		client := testlib.Setup(t, true, testlib.SetupClientOptionNoop)
		defer testlib.TearDown(client)

		helpers.BrokerV1ControlPlaneTest(
			t,
			func(client *testlib.Client, name string) {
				helpers.BrokerDataPlaneSetupHelper(context.Background(), client, brokerClass, brokerTestRunner)
			},
			testlib.SetupClientOptionNoop,
		)
	})
}
