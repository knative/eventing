package helpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
	"testing"
)

func SourceCRDMetadataTestHelperWithChannelTestRunner(
	t *testing.T,
	sourceTestRunner lib.ComponentsTestRunner,
	options ...lib.SetupClientOption,
) {

	sourceTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		t.Run("Source CRD has required label", func(t *testing.T) {
			sourceCRDHasRequiredLabels(st, client, source)
		})

	})
}

func sourceCRDHasRequiredLabels(st *testing.T, client *lib.Client, source metav1.TypeMeta) {
	// From spec:
	// Each source MUST have the following:
	//   label of duck.knative.dev/source: "true"

	gvr, _ := meta.UnsafeGuessKindToResource(source.GroupVersionKind())
	crdName := gvr.Resource + "." + gvr.Group

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{},
	})
	if err != nil {
		client.T.Fatalf("Unable to find CRD for %q: %v", source, err)
	}
	if crd.Labels["duck.knative.dev/source"] != "true" {
		client.T.Fatalf("Source CRD doesn't have the label 'duck.knative.dev/source=true' %q: %v", source, err)
	}
}
