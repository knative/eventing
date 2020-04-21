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
	"fmt"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	pkgTest "knative.dev/pkg/test"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

func TestListCRDs(t *testing.T) {
	client, err := lib.NewClient(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		"default",
		t)
	if err != nil {
		t.Fatalf("Can't create client %v", err)
	}

	client.T.Logf("Kubeconfig: %q", pkgTest.Flags.Kubeconfig)
	client.T.Logf("Cluster: %q", pkgTest.Flags.Cluster)

	allCrds, err := client.Apiextensions.CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		client.T.Logf("LIST CRDs failed. Err is : %v", err)
		client.T.Fatalf("LIST CRDs failed. Err is : %v", err)
	} else {
		client.T.Logf("allCrds is : %v", len(allCrds.Items))
		for _, c := range allCrds.Items {
			client.T.Logf("c is : %v ### %v ### %v\n", c.Spec.Group, c.Spec.Names, crdVersionToShortString(c.Spec.Versions))
		}
	}
}

func TestGetCRD1(t *testing.T) {
	client, err := lib.NewClient(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		"default",
		t)
	if err != nil {
		t.Fatalf("Can't create client %v", err)
	}

	singleCrd, err := client.Apiextensions.CustomResourceDefinitions().Get("inmemorychannels.messaging.knative.dev", metav1.GetOptions{})
	if err != nil {
		client.T.Logf("GET CRD failed. Err is : %v", err)
		client.T.Fatalf("GET CRD failed. Err is : %v", err)
	} else {
		client.T.Logf("c is : %v ### %v ### %v\n", singleCrd.Spec.Group, singleCrd.Spec.Names, crdVersionToShortString(singleCrd.Spec.Versions))
	}
}

func TestGetCRD2(t *testing.T) {
	client, err := lib.NewClient(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		"default",
		t)
	if err != nil {
		t.Fatalf("Can't create client %v", err)
	}

	singleCrd, err := client.Apiextensions.CustomResourceDefinitions().Get("inmemorychannel.messaging.knative.dev", metav1.GetOptions{})
	if err != nil {
		client.T.Logf("GET CRD failed. Err is : %v", err)
		client.T.Fatalf("GET CRD failed. Err is : %v", err)
	} else {
		client.T.Logf("c is : %v ### %v ### %v\n", singleCrd.Spec.Group, singleCrd.Spec.Names, crdVersionToShortString(singleCrd.Spec.Versions))
	}
}

func crdVersionToShortString(arr []apiextensionsv1beta1.CustomResourceDefinitionVersion) string {
	ret := ""
	for _, v := range arr {
		ret += fmt.Sprintf("%v %v %v, ", v.Name, v.Served, v.Storage)
	}
	return ret
}
