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

package duck

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/eventtype/fake"
	_ "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/source/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	gvr := schema.GroupVersionResource{
		Group:    "testing.sources.knative.dev",
		Version:  "v1alpha1",
		Resource: "testsources",
	}
	gvk := schema.GroupVersionKind{
		Group:   "testing.sources.knative.dev",
		Version: "v1alpha1",
		Kind:    "TestSource",
	}
	crd := "testsources.testing.sources.knative.dev"

	c := NewController(crd, gvr, gvk)(ctx, configmap.NewStaticWatcher())
	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
