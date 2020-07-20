// +build e2e

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

package e2e

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sugarresources "knative.dev/eventing/pkg/reconciler/sugar/resources"
	"knative.dev/eventing/test/e2e/helpers"
	testlib "knative.dev/eventing/test/lib"
)

var unsupportedChannelVersions = []string{"v1alpha1"}

func DefaultBrokerCreator(_ *testlib.Client, _ string) string {
	return sugarresources.DefaultBrokerName
}

func TestDefaultBrokerWithManyTriggers(t *testing.T) {
	helpers.TestBrokerWithManyTriggers(t, DefaultBrokerCreator, true)
}

func TestChannelBasedBrokerWithManyTriggers(t *testing.T) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		for _, version := range unsupportedChannelVersions {
			if strings.HasSuffix(channel.APIVersion, version) {
				t.Skipf("unsupported %s channel version", version)
			}
		}

		brokerCreator := helpers.ChannelBasedBrokerCreator(channel, brokerClass)

		helpers.TestBrokerWithManyTriggers(t, brokerCreator, false)
	})
}
