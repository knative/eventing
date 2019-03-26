/*
Copyright 2018 The Knative Authors

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

package subscription

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestProvideController(t *testing.T) {
	// TODO(grantr) This needs a mock of manager.Manager. Creating a manager
	// with a fake Config fails because the Manager tries to contact the
	// apiserver.

	// cfg := &rest.Config{
	// 	Host: "http://foo:80",
	// }
	//
	// mgr, err := manager.New(cfg, manager.Options{})
	// if err != nil {
	// 	t.Fatalf("Error creating manager: %v", err)
	// }
	//
	// _, err = ProvideController(mgr)
	// if err != nil {
	// 	t.Fatalf("Error in ProvideController: %v", err)
	// }
}

func TestInjectClient(t *testing.T) {
	r := &reconciler{}
	orig := r.client
	n := fake.NewFakeClient()
	if orig == n {
		t.Errorf("Original and new clients are identical: %v", orig)
	}
	err := r.InjectClient(n)
	if err != nil {
		t.Errorf("Unexpected error injecting the client: %v", err)
	}
	if n != r.client {
		t.Errorf("Unexpected client. Expected: '%v'. Actual: '%v'", n, r.client)
	}
}

func TestInjectConfig(t *testing.T) {
	r := &reconciler{}
	wantCfg := &rest.Config{
		Host: "http://foo",
	}

	err := r.InjectConfig(wantCfg)
	if err != nil {
		t.Fatalf("Unexpected error injecting the config: %v", err)
	}

	gotCfg := r.restConfig
	if diff := cmp.Diff(wantCfg, gotCfg); diff != "" {
		t.Errorf("Unexpected config (-want, +got): %v", diff)
	}

	wantDynClient, err := dynamic.NewForConfig(wantCfg)
	if err != nil {
		t.Fatalf("Unexpected error generating dynamic client: %v", err)
	}

	// Since dynamicClient doesn't export any fields, we can only test its type.
	switch r.dynamicClient.(type) {
	case dynamic.Interface:
		// ok
	default:
		t.Errorf("Unexpected dynamicClient type. Expected: %T, Got: %T", wantDynClient, r.dynamicClient)
	}
}
