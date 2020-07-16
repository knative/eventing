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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/apis"
	"testing"
)
func SetupApiServerSource(c *testlib.Client){

}
func SetupPingSource(c *testlib.Client){

}
func SetupContainerSource(c *testing..Client){

}
func TestSourcesStatusConformance(t *testing.T) {
	SourceStatusTestHelperWithComponentsTestRunner(t,
		sourcesTestRunner,
		SetupApiServerSource)
}

func SourceStatusTestHelperWithComponentsTestRunner(
	t *testing.T,
	componentsTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {
	table := [] struct {
		name    string
		feature testlib.Feature
		// Sources report success via either a Ready condition or a Succeeded condition
		want    apis.ConditionType
	} {
		{
			"Long lived sources have Ready status condition",
			testlib.FeatureLongLived,
			apis.ConditionReady,
		},
		{
			"Batch sources have Succeeded status condition",
			testlib.FeatureBatch,
			apis.ConditionSucceeded,
		},

	}
	for _, tc := range table {
		componentsTestRunner.RunTests(t, tc.feature, func(st *testing.T, source metav1.TypeMeta) {
			client := testlib.Setup(st, true, options...)
			defer testlib.TearDown(client)

			t.Run(tc.name, func(t *testing.T) {
				validateSourceStatus(st, client, source, tc.want, options...)
			})
		})
	}
}

func validateSourceStatus(st *testing.T, client *testlib.Client, source metav1.TypeMeta, successCondition apis.ConditionType, options ...testlib.SetupClientOption) {
	const(
		sourceName = "source-req-status"
	)

	st.Logf("Running source status conformance test with source %q", source)

	client.T.Logf("Creating source %+v-%s", source, sourceName)
	//client.CreateSourceOrFail(sourceName, &source)
	//client.WaitForResourceReadyOrFail(sourceName, &source)

	dtsv, err := getSourceDuckTypeSupportedVersion(sourceName, client, &source)
	if err != nil {
		st.Fatalf("Unable to check source duck type supported version for %q: %q", source, err)
	}

	if dtsv == "" || dtsv == "v1alpha1" {
		// treat missing annotation value as v1alpha1, as written in the spec
		versionedSrc, err := getSourceAsV1Alpha1Source(sourceName, client, source)
		if err != nil {
			st.Fatalf("Unable to get source %q with v1alpha1 duck type: %q", source, err)
		}

		// SPEC: Sources MUST implement conditions with a Ready condition for long lived sources, and Succeeded for batch style sources.
		if ! hasCondition(versionedSrc, successCondition){
			st.Fatalf("%q does not have %q", source, successCondition)
		}

		// SPEC: Sources MUST propagate the sinkUri to their status to signal to the cluster where their events are being sent.
		if versionedSrc.Status.SinkURI.Host == "" {
			st.Fatalf("sinkUri was not propagated for source %q", source)
		}
	} else if dtsv == "v1alpha2" {
		versionedSrc, err := getSourceAsV1Alpha2Source(sourceName, client, source)
		if err != nil {
			st.Fatalf("Unable to get source %q with v1alpha2 duck type: %q", source, err)
		}
		// SPEC: Sources MUST implement conditions with a Ready condition for long lived sources, and Succeeded for batch style sources.
		if ! hasCondition(versionedSrc, successCondition){
			st.Fatalf("%q does not have %q", source, successCondition)
		}

		// SPEC: Sources MUST propagate the sinkUri to their status to signal to the cluster where their events are being sent.
		if versionedSrc.Status.SinkURI.Host == "" {
			st.Fatalf("sinkUri was not propagated for source %q", source)
		}

	} else {
		st.Fatalf("Source doesn't support v1alpha1 or v1alpha2 Source duck types: %q", source)
	}
}
