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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

// ChannelCRDMetadataTestHelperWithChannelTestRunner runs the Channel CRD metadata tests for all
// Channel resources in the ChannelTestRunner.
func ChannelCRDMetadataTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		t.Run("Channel is namespaced", func(t *testing.T) {
			channelIsNamespaced(st, client, channel)
		})
		t.Run("Channel CRD has required label", func(t *testing.T) {
			channelCRDHasRequiredLabels(st, client, channel)
		})
		t.Run("Channel CRD has required label", func(t *testing.T) {
			channelCRDHasProperCategory(st, client, channel)
		})
	})
}

func channelIsNamespaced(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	// From spec: Each channel is namespaced

	apiResource, err := getApiResource(client, channel)
	if err != nil {
		client.T.Fatalf("Error finding server resource for %q: %v", channel, err)
	}
	if !apiResource.Namespaced {
		client.T.Fatalf("%q is not namespace scoped: %v", channel, err)
	}
}

func channelCRDHasRequiredLabels(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	// From spec:
	// Each channel MUST have the following:
	//   label of messaging.knative.dev/subscribable: "true"
	//   label of duck.knative.dev/addressable: "true"

	gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())
	crdName := gvr.Resource + "." + gvr.Group

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{},
	})
	if err != nil {
		client.T.Fatalf("Unable to find CRD for %q: %v", channel, err)
	}
	if crd.Labels["messaging.knative.dev/subscribable"] != "true" {
		client.T.Fatalf("Channel CRD doesn't have the label 'messaging.knative.dev/subscribable=true' %q: %v", channel, err)
	}
	if crd.Labels["duck.knative.dev/addressable"] != "true" {
		client.T.Fatalf("Channel CRD doesn't have the label 'duck.knative.dev/addressable=true' %q: %v", channel, err)
	}
}

func channelCRDHasProperCategory(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	// From spec:
	// Each channel MUST have the following: the category channel

	apiResource, err := getApiResource(client, channel)
	if err != nil {
		client.T.Fatalf("Error finding server resource for %q: %v", channel, err)
	}
	found := false
	for _, cat := range apiResource.Categories {
		if cat == "channel" {
			found = true
			break
		}
	}
	if !found {
		client.T.Fatalf("Channel CRD %q does not have the category 'channel': %v", channel, err)
	}
}

func getApiResource(client *lib.Client, typeMeta metav1.TypeMeta) (*metav1.APIResource, error) {
	gvr, _ := meta.UnsafeGuessKindToResource(typeMeta.GroupVersionKind())
	apiResourceList, err := client.Kube.Kube.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to list server resources for groupVersion of %q: %v", typeMeta, err)
	}

	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Kind == typeMeta.Kind {
			return &apiResource, nil
		}
	}
	return nil, errors.Errorf("Unable to find server resource for %q: %v", typeMeta, err)
}
