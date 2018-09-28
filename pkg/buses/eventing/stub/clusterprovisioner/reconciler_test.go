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

package clusterprovisioner

import (
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	controllertesting "github.com/knative/eventing/pkg/controller/testing"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestInjectClient(t *testing.T) {
	r :=  &reconciler{}
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

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "CP not found",
		},
		{
			Name: "Unable to get CP",
		},
		{
			Name: "Should not reconcile",
		},
		{
			Name: "Delete provisioner fails",
		},
		{
			Name: "Delete dispatcher fails",
		},
		{
			Name: "Delete succeeds",
		},
		{
			Name: "Creates dispatcher fails",
		},
		{
			Name: "Creates dispatcher - already exists",
		},
		{
			Name: "Creates dispatcher - now owned by CP",
		},
		{
			Name: "Creates dispatcher succeeds",
		},
		{
			Name: "Sync provisioners - fails",
		},
		{
			Name: "Sync provisioners",
		},
	}
}
