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

package apiserver

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	reconcilertesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"

	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"

	_ "knative.dev/pkg/system/testing"
)

const (
	fakeHost = "unit-test-k8s-host"
)

func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = fakekubeclientset.AddToScheme(scheme)
	return scheme
}

// SetupFakeContextWithCancel sets up the the Context and the fake informers for the tests
// The provided context can be canceled using provided callback.
func SetupFakeContextWithCancel(t *testing.T, objects []runtime.Object) (context.Context, context.CancelFunc) {
	ctx, c := context.WithCancel(logtesting.TestContextWithLogger(t))
	cfg := &rest.Config{Host: fakeHost}
	ctx = withStoredHost(ctx, cfg)
	ctx, _ = injection.Fake.SetupInformers(ctx, cfg)
	ctx, _ = fakedynamicclient.With(ctx, NewScheme(), reconcilertesting.ToUnstructured(t, objects)...)
	return ctx, c
}

func TestNewAdaptor(t *testing.T) {
	ce := adaptertest.NewTestClient()

	testCases := map[string]struct {
		opt     envConfig
		source  string
		objects []runtime.Object
	}{
		"empty": {
			source: fakeHost,
			opt: envConfig{
				ConfigJson: "{}",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx, _ := SetupFakeContextWithCancel(t, tc.objects)
			a := NewAdapter(ctx, &tc.opt, ce)

			got, ok := a.(*apiServerAdapter)
			if !ok {
				t.Errorf("expected NewAdapter to return a *adapter, but did not")
			}
			if got == nil {
				t.Fatalf("expected NewAdapter to return a *adapter, but got nil")
			}

			if got.source != tc.source {
				t.Errorf("expected source to be %s, got %s", tc.source, got.source)
			}

		})
	}
}
