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

package pingsource

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/feature"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/tracing/config"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta2/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1/pingsource/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/fake"
	. "knative.dev/pkg/reconciler/testing"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	c := NewController(ctx, configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metrics.ConfigMapName(),
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      logging.ConfigMapName(),
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"zap-logger-config":   "test-config",
				"loglevel.controller": "info",
				"loglevel.webhook":    "info",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ConfigName,
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      feature.FlagsConfigName,
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config-kreference-mapping",
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		},
	))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
