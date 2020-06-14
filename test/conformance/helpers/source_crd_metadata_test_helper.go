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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

func SourceCRDMetadataTestHelperWithChannelTestRunner(
	t *testing.T,
	sourceTestRunner lib.ComponentsTestRunner,
	options ...lib.SetupClientOption,
) {

	sourceTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// From spec:
		// Each source MUST have the following:
		//   label of duck.knative.dev/source: "true"
		t.Run("Source CRD has required label", func(t *testing.T) {
			yes, err := objectHasRequiredLabels(client, source, "duck.knative.dev/source", "true")
			if err != nil {
				client.T.Fatalf("Unable to find CRD for %q: %v", source, err)
			}
			if !yes {
				client.T.Fatalf("Source CRD doesn't have the label 'duck.knative.dev/source=true' %q: %v", source, err)
			}
		})

	})
}
