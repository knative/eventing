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

package apiserversource

import (
	"context"
	"os"
	"testing"

	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"

	"knative.dev/eventing/pkg/apis/sources"
	"knative.dev/eventing/pkg/eventingtls"

	"knative.dev/eventing/pkg/apis/feature"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/tracing/config"

	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	// Fake injection informers
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/role/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/filtered/fake"
	. "knative.dev/pkg/reconciler/testing"

	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1/apiserversource/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

	ctx = withCfgHost(ctx, &rest.Config{Host: "unit_test"})
	ctx = addressable.WithDuck(ctx)

	os.Setenv("METRICS_DOMAIN", "knative.dev/eventing")
	os.Setenv("APISERVER_RA_IMAGE", "knative.dev/example")
	c := NewController(ctx, configmap.NewStaticWatcher(&corev1.ConfigMap{
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
	}))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func SetUpInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, eventingtls.TrustBundleLabelSelector, sources.OIDCTokenRoleLabelSelector)
	return ctx
}
