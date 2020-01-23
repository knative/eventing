/*
Copyright 2020 The Knative Authors

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

package pingsource

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha1/pingsource/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
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
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			if tc.setEnv {
				if err := os.Setenv("PING_IMAGE", "anything"); err != nil {
					t.Fatalf("Failed to set env var: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("PING_IMAGE"); err != nil {
						t.Fatalf("Failed to unset env var: %v", err)
					}
				}()

				if err := os.Setenv("METRICS_DOMAIN", "knative.dev/eventing"); err != nil {
					t.Fatalf("Failed to set env var: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("METRICS_DOMAIN"); err != nil {
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
			c := NewController(ctx, configmap.NewStaticWatcher(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config-observability",
						Namespace: "knative-eventing",
					},
					Data: map[string]string{
						"_example": "test-config",
					},
				}, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config-logging",
						Namespace: "knative-eventing",
					},
					Data: map[string]string{
						"zap-logger-config":   "test-config",
						"loglevel.controller": "info",
						"loglevel.webhook":    "info",
					},
				},
			))

			if c == nil {
				t.Fatal("Expected NewController to return a non-nil value")
			}
		})
	}
}
