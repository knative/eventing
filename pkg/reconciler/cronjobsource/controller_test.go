/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cronjobsource

import (
	"os"
	"testing"

	"knative.dev/pkg/configmap"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha1/cronjobsource/fake"
	_ "knative.dev/pkg/injection/informers/kubeinformers/appsv1/deployment/fake"
)

func TestNew(t *testing.T) {
	testCases := map[string]struct {
		setEnv bool
	}{
		"image not set": {},
		"image set": {
			setEnv: true,
		},
	}
	defer logtesting.ClearAll()
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			if tc.setEnv {
				if err := os.Setenv("CRONJOB_RA_IMAGE", "anything"); err != nil {
					t.Fatalf("Failed to set env var: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("CRONJOB_RA_IMAGE"); err != nil {
						t.Fatalf("Failed to unset env var: %v", err)
					}
				}()
			} else {
				defer func() {
					r := recover()
					if r == nil {
						t.Errorf("Expected NewController to panic, nothing recovered.")
					}
				}()
			}

			ctx, _ := SetupFakeContext(t)
			c := NewController(ctx, configmap.NewFixedWatcher())

			if c == nil {
				t.Fatal("Expected NewController to return a non-nil value")
			}
		})
	}
}
