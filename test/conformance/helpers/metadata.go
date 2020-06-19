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
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
)

func ValidateRequiredLabels(client *testlib.Client, object metav1.TypeMeta, labels map[string]string) {
	for k, v := range labels {
		if !objectHasRequiredLabel(client, object, k, v) {
			client.T.Fatalf("can't find label '%s=%s' in CRD %q", k, v, object)
		}
	}
}

func objectHasRequiredLabel(client *testlib.Client, object metav1.TypeMeta, key string, value string) bool {
	gvr, _ := meta.UnsafeGuessKindToResource(object.GroupVersionKind())
	crdName := gvr.Resource + "." + gvr.Group

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{},
	})
	if err != nil {
		client.T.Errorf("error while getting %q:%v", object, err)
	}
	return crd.Labels[key] == value
}
