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
	"encoding/json"

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

type EventTypesAnnotationJsonValue struct {
	Type        string `json:"type"`
	Schema      string `json:"schema,omitempty"`
	Description string `json:"description,omitempty"`
}

func ValidateAnnotations(client *testlib.Client, object metav1.TypeMeta, annotationsKey string) {
	if !objectHasValidAnnotation(client, object, annotationsKey) {
		//CRD Spec says Source CRDs "SHOULD" use a standard annotation - so can't enforce it. Nothing to do here
		//client.T.Fatalf("can't find annotation '%s' in CRD %q", annotationsKey, object)
		client.T.Logf("can't find annotation '%s' in CRD %q", annotationsKey, object)
	}
}

func objectHasValidAnnotation(client *testlib.Client, object metav1.TypeMeta, key string) bool {
	gvr, _ := meta.UnsafeGuessKindToResource(object.GroupVersionKind())
	crdName := gvr.Resource + "." + gvr.Group

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{},
	})
	if err != nil {
		client.T.Errorf("error while getting %q:%v", object, err)
	}

	if annotationsValue, found := crd.Annotations[key]; found {
		client.T.Logf("found annotation key %s in object %q", key, object)
		if !annotationIsValidJSONArray(client, annotationsValue) {
			client.T.Fatalf("annotation value %s in CRD %q does not meet CRD Specs", annotationsValue, object)
		}
		return true
	}
	return false
}

func annotationIsValidJSONArray(client *testlib.Client, value string) bool {
	if !json.Valid([]byte(value)) {
		client.T.Fatalf("invalid JSON array in annotation %s", value)
		return false
	}

	var valueUnmarshalled []EventTypesAnnotationJsonValue
	if err := json.Unmarshal([]byte(value), &valueUnmarshalled); err != nil {
		client.T.Fatalf("error unmarshalling JSON array %s to Go type", value)
		return false
	}

	//Each object contain fields type: String (Mandatory) schema: String (Optional) description: String (Optional)
	for _, valueObject := range valueUnmarshalled {
		if valueObject.Type == "" {
			client.T.Fatal("required Type field in JSON object is empty")
			return false
		}
	}

	return true
}
