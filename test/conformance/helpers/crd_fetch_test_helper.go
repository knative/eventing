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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

func CRDFetchTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)
		t.Run("TEST GET OR LIST CRDS", func(t *testing.T) {
			getOrListCRDs(st, client, channel)
		})
	})
}

func getOrListCRDs(st *testing.T, client *lib.Client, channel metav1.TypeMeta) {
	gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())
	client.T.Logf("gvr is : %v", gvr)
	crdName := gvr.Resource + "." + gvr.Group
	client.T.Logf("crdName is : %v", crdName)

	allCrds, err := client.Apiextensions.CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		client.T.Logf("LIST CRDs failed. Err is : %v", err)
	} else {
		client.T.Logf("allCrds is : %v", len(allCrds.Items))
		for _, c := range allCrds.Items {
			client.T.Logf("c is : %v ### %v ### %v\n", c.Spec.Group, c.Spec.Names, crdVersionToShortString(c.Spec.Versions))
		}
	}

	singleCrd, err := client.Apiextensions.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
	if err != nil {
		client.T.Logf("GET CRD failed. Err is : %v", err)
	} else {
		client.T.Logf("crd is : %v", singleCrd)
	}
}

func crdVersionToShortString(arr []v1.CustomResourceDefinitionVersion) string {
	ret := ""
	for _, v := range arr {
		ret += fmt.Sprintf("%v %v %v, ", v.Name, v.Served, v.Storage)
	}
	return ret
}
