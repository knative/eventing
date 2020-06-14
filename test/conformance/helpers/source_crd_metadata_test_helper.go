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
