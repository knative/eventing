/*
Copyright 2021 The Knative Authors

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

package v1beta1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestKind(t *testing.T) {
	schemaGroupKind := Kind("EventType")
	if schemaGroupKind.Kind != "EventType" || schemaGroupKind.Group != "eventing.knative.dev" {
		t.Errorf("Unexpected GroupKind: %+v", schemaGroupKind)
	}
}

func TestResource(t *testing.T) {
	schemaGroupResource := Resource("EventType")
	if schemaGroupResource.Group != "eventing.knative.dev" || schemaGroupResource.Resource != "EventType" {
		t.Errorf("Unexpected GroupResource: %+v", schemaGroupResource)
	}
}

func TestKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	addKnownTypes(scheme)
	types := scheme.KnownTypes(SchemeGroupVersion)

	for _, name := range []string{
		"EventType",
		"EventTypeList",
	} {
		if _, ok := types[name]; !ok {
			t.Errorf("Did not find %q as registered type", name)
		}
	}
}
