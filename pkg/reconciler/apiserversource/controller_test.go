/*
Copyright 2019 The Knative Authors

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

package apiserversource

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha1/apiserversource/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	ctx = withCfgHost(ctx, &rest.Config{Host: "unit_test"})
	os.Setenv("METRICS_DOMAIN", "knative.dev/eventing")
	os.Setenv("APISERVER_RA_IMAGE", "knative.dev/example")
	c := NewController(ctx, configmap.NewStaticWatcher(&corev1.ConfigMap{
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
	}))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
