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
	"fmt"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

// ChannelMetadataTestHelperWithChannelTestRunner runs the Channel metadata tests for all Channels in
// the ChannelTestRunner.
func ChannelMetadataTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		//t.Run("Channel is namespaced", func(t *testing.T) {
		//	channelIsNamespaced(st, client, channel)
		//})
		t.Run("Channel has required label", func(t *testing.T) {
			channelCRDHasSubscribableLabel(st, client, channel)
		})
		//t.Run("Channel has required label", func(t *testing.T) {
		//	channelHasProperCategory(st, client, channel)
		//})
	})
}

func channelIsNamespaced(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	apiResource, err := getApiResource(client, channel)
	if err != nil {
		client.T.Fatalf("Error finding server resource for %q: %v", channel, err)
	}
	if !apiResource.Namespaced {
		client.T.Fatalf("%q is not namespace scoped: %v", channel, err)
	}
}

func channelCRDHasSubscribableLabel(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())
	client.T.Logf("gvr is : %v", gvr)
	crdName := gvr.Resource + "." + gvr.Group
	client.T.Logf("crdName is : %v", crdName)

	allCrds, err := client.Apiextensions.CustomResourceDefinitions().List(metav1.ListOptions{})
	client.T.Logf("err is : %v", err)
	client.T.Logf("allCrds is : %v", len(allCrds.Items))

	for _, c := range allCrds.Items {
		client.T.Logf("c is : %v ### %v ### %v\n", c.Spec.Group, c.Spec.Names, getSomeText(c.Spec.Versions))
	}

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
	//client.T.Logf("crd is : %v", crd)
	client.T.Logf("err is : %v", err)
	if err != nil {
		client.T.Fatalf("Unable to find CRD for %q: %v", channel, err)
	}
	client.T.Logf("crd.Labels is : %v", crd.Labels)
	if crd.Labels["messaging.knative.dev/subscribable"] != "true" {
		client.T.Fatalf("Channel doesn't have the label 'messaging.knative.dev/subscribable=true' %q: %v", channel, err)
	}
}

func getSomeText(arr []v1.CustomResourceDefinitionVersion) string {
	ret := ""

	for _, v := range arr {
		ret += fmt.Sprintf("%v %v %v, ", v.Name, v.Served, v.Storage)

	}

	return ret
}

func channelHasProperCategory(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
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
		client.T.Fatalf("%q does not have the category 'channel': %v", channel, err)
	}
}

func getApiResource(client *lib.Client, typeMeta metav1.TypeMeta) (*metav1.APIResource, error) {
	gvr, _ := meta.UnsafeGuessKindToResource(typeMeta.GroupVersionKind())
	client.T.Logf("gvr is : %v", gvr)
	apiResourceList, err := client.Kube.Kube.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	client.T.Logf("apiResourceList is : %v", apiResourceList)
	client.T.Logf("err is : %v", err)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to list server resources for groupVersion of %q: %v", typeMeta, err)
	}

	for _, apiResource := range apiResourceList.APIResources {
		client.T.Logf("apiResource is : %v", apiResource)
		if apiResource.Kind == typeMeta.Kind {
			client.T.Logf("found apiResource is : %v", apiResource)
			return &apiResource, nil
		}
	}
	client.T.Logf("No apiResource found")
	return nil, errors.Errorf("Unable to find server resource for %q: %v", typeMeta, err)
}
