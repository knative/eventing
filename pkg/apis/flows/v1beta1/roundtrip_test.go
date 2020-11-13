/*
Copyright 2020 The Knative Authors.

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

package v1beta1

import (
	"math/rand"
	"testing"

	fuzz "github.com/google/gofuzz"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	pkgfuzzer "knative.dev/pkg/apis/testing/fuzzer"
	"knative.dev/pkg/apis/testing/roundtrip"
)

var (
	linear      = duckv1.BackoffPolicyLinear
	exponential = duckv1.BackoffPolicyExponential
	bops        = []*duckv1.BackoffPolicyType{nil, &linear, &exponential}
)

// FuzzerFuncs includes fuzzing funcs for knative.dev/flows v1 types
//
// For other examples see
// https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/fuzzer/fuzzer.go
var FuzzerFuncs = fuzzer.MergeFuzzerFuncs(
	func(codecs serializer.CodecFactory) []interface{} {
		return []interface{}{
			func(s *v1.SequenceStatus, c fuzz.Continue) {
				c.FuzzNoCustom(s) // fuzz the status object

				// Clear the random fuzzed condition
				s.Status.SetConditions(nil)

				// Fuzz the known conditions except their type value
				s.InitializeConditions()
				pkgfuzzer.FuzzConditions(&s.Status, c)
			},
			func(s *v1.ParallelStatus, c fuzz.Continue) {
				c.FuzzNoCustom(s) // fuzz the status object

				// Clear the random fuzzed condition
				s.Status.SetConditions(nil)

				// Fuzz the known conditions except their type value
				s.InitializeConditions()
				pkgfuzzer.FuzzConditions(&s.Status, c)
			},
			func(ds *duckv1.DeliverySpec, c fuzz.Continue) {
				c.FuzzNoCustom(ds) // fuzz the DeliverySpec
				if ds.BackoffPolicy != nil && *ds.BackoffPolicy == "" {
					ds.BackoffPolicy = nil
				} else {
					//nolint:gosec // Cryptographic randomness is not necessary.
					ds.BackoffPolicy = bops[rand.Intn(3)]
				}
			},
		}
	},
)

func TestFlowsRoundTripTypesToJSON(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(AddToScheme(scheme))

	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(
		pkgfuzzer.Funcs,
		FuzzerFuncs,
	)
	roundtrip.ExternalTypesViaJSON(t, scheme, fuzzerFuncs)
}

func TestFlowsRoundTripTypesToBetaHub(t *testing.T) {
	scheme := runtime.NewScheme()

	sb := runtime.SchemeBuilder{
		AddToScheme,
		v1.AddToScheme,
	}

	utilruntime.Must(sb.AddToScheme(scheme))

	hubs := runtime.NewScheme()
	hubs.AddKnownTypes(SchemeGroupVersion,
		&Parallel{},
		&Sequence{},
	)

	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(
		pkgfuzzer.Funcs,
		FuzzerFuncs,
	)

	roundtrip.ExternalTypesViaHub(t, scheme, hubs, fuzzerFuncs)
}
