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

package sources

import (
	"testing"

	"knative.dev/eventing/test/conformance/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	testlib "knative.dev/eventing/test/lib"
)

func SourceCRDRegistryTestHelperWithChannelTestRunner(
	t *testing.T,
	sourceTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {

	sourceTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// From spec:
		// Source CRDs SHOULD use a standard annotation to expose the types of events.
		// If specified, the annotation MUST be a valid JSON array.
		// Each object in the array MUST contain the following fields:
		// 	- type: String. Mandatory.
		// 	- schema: String. Optional.
		// 	- description: String. Optional.

		st.Run("Source CRD has required annotations", func(t *testing.T) {
			helpers.ValidateAnnotations(client, source, eventing.EventTypesAnnotationKey)
		})

	})
}
