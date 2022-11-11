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
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// SourceStatusTestHelperWithComponentsTestRunner runs the Source status
// conformance tests for all sources in the ComponentsTestRunner. This test
// needs an already created instance of each source which should be initialized
// via ComponentsTestRunner.AddComponentSetupClientOption.
//
// Note: The source object name must be the lower case Kind name (e.g.
// apiserversource for the Kind: ApiServerSource source)
func SourceStatusTestHelperWithComponentsTestRunner(
	t *testing.T,
	componentsTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {
	table := []struct {
		name    string
		feature testlib.Feature
		// Sources report success via either a Ready condition or a Succeeded condition
		want apis.ConditionType
	}{
		{
			name:    "Long living sources have Ready status condition",
			feature: testlib.FeatureLongLiving,
			want:    apis.ConditionReady,
		},
		{
			name:    "Batch sources have Succeeded status condition",
			feature: testlib.FeatureBatch,
			want:    apis.ConditionSucceeded,
		},
	}

	for _, tc := range table {
		n := tc.name
		f := tc.feature
		w := tc.want
		t.Run(n, func(t *testing.T) {
			componentsTestRunner.RunTestsWithComponentOptions(t, f, true,
				func(st *testing.T, source metav1.TypeMeta,
					componentOptions ...testlib.SetupClientOption) {
					st.Log("About to setup client")
					options = append(options, componentOptions...)
					client := testlib.Setup(st, true, options...)
					defer testlib.TearDown(client)
					validateSourceStatus(st, client, source, w)
				})
		})
	}
}

func validateSourceStatus(st *testing.T, client *testlib.Client,
	source metav1.TypeMeta,
	successCondition apis.ConditionType) {

	st.Logf("Running source status conformance test with source %q", source)

	v1beta1Src, err := getSourceAsV1Beta1Source(client, source)
	if err != nil {
		st.Fatalf("Unable to get source %q with v1beta1 duck type: %v", source, err)
	}

	// SPEC: Sources MUST implement conditions with a Ready condition for long lived sources, and Succeeded for batch style sources.
	if !hasCondition(v1beta1Src, successCondition) {
		st.Fatalf("Source %q does not have condition %q", source, successCondition)
	}

	// SPEC: Sources MUST propagate the sinkUri to their status to signal to the cluster where their events are being sent.
	if v1beta1Src.Status.SinkURI.Host == "" {
		st.Fatalf("sinkUri was not propagated for source %q", source)
	}
}

func getSourceAsV1Beta1Source(client *testlib.Client,
	source metav1.TypeMeta) (*duckv1beta1.Source, error) {
	srcName := strings.ToLower(source.Kind)
	metaResource := resources.NewMetaResource(srcName, client.Namespace,
		&source)
	obj, err := duck.GetGenericObject(client.Dynamic, metaResource,
		&duckv1beta1.Source{})
	if err != nil {
		return nil, fmt.Errorf("unable to get the source as v1beta1 "+
			"Source duck type: %q: %w", source, err)
	}
	srcObj, ok := obj.(*duckv1beta1.Source)
	if !ok {
		return nil, fmt.Errorf("unable to cast source %q to v1beta1 "+
			"Source duck type", source)
	}
	return srcObj, nil
}

func hasCondition(src *duckv1beta1.Source, t apis.ConditionType) bool {
	return src.Status.GetCondition(t) != nil
}
