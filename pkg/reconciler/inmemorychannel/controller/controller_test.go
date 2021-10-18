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

package controller

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"

	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"

	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel/fake"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/config"

	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/service/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	ctx = v1addr.WithDuck(ctx)

	os.Setenv("DISPATCHER_IMAGE", "animage")
	cmw := configmap.NewStaticWatcher(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EventDispatcherConfigMap,
			Namespace: "knative-eventing",
		},
	})
	c := NewController(ctx, cmw)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
