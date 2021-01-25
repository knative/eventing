/*
Copyright 2021 The Knative Authors

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

package v1

import (
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	fuzz "github.com/google/gofuzz"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	pkgfuzzer "knative.dev/pkg/apis/testing/fuzzer"
	"knative.dev/pkg/apis/testing/roundtrip"
)

// FuzzerFuncs includes fuzzing funcs for knative.dev/sources v1 types
//
// For other examples see
// https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/fuzzer/fuzzer.go
var FuzzerFuncs = fuzzer.MergeFuzzerFuncs(
	func(codecs serializer.CodecFactory) []interface{} {
		return []interface{}{
			func(source *ApiServerSource, c fuzz.Continue) {
				c.FuzzNoCustom(source) // fuzz the source
				// Clear the random fuzzed condition
				source.Status.SetConditions(nil)

				// Fuzz the known conditions except their type value
				source.Status.InitializeConditions()
				pkgfuzzer.FuzzConditions(&source.Status, c)
			},
			func(source *ContainerSource, c fuzz.Continue) {
				c.FuzzNoCustom(source) // fuzz the source
				// Clear the random fuzzed condition
				source.Status.SetConditions(nil)

				// Fuzz the known conditions except their type value
				source.Status.InitializeConditions()
				pkgfuzzer.FuzzConditions(&source.Status, c)
			},
			func(source *SinkBinding, c fuzz.Continue) {
				c.FuzzNoCustom(source) // fuzz the source
				// Clear the random fuzzed condition
				source.Status.SetConditions(nil)

				// Fuzz the known conditions except their type value
				source.Status.InitializeConditions()
				pkgfuzzer.FuzzConditions(&source.Status, c)
			},
		}
	},
)

func TestSourcesRoundTripTypesToJSON(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(AddToScheme(scheme))

	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(
		pkgfuzzer.Funcs,
		FuzzerFuncs,
	)
	roundtrip.ExternalTypesViaJSON(t, scheme, fuzzerFuncs)
}
