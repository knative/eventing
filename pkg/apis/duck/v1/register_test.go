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
package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kind takes an unqualified resource and returns a Group qualified GroupKind
func TestKind(t *testing.T) {
	want := schema.GroupKind{
		Group: "duck.knative.dev",
		Kind:  "kind",
	}

	got := Kind("kind")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected resource (-want, +got) =", diff)
	}
}

// TestKnownTypes makes sure that expected types get added.
func TestKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	addKnownTypes(scheme)
	types := scheme.KnownTypes(SchemeGroupVersion)

	for _, name := range []string{
		"Channelable",
		"ChannelableList",
	} {
		if _, ok := types[name]; !ok {
			t.Errorf("Did not find %q as registered type", name)
		}
	}
}
