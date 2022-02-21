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

package trigger

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	"knative.dev/eventing/pkg/apis/sugar"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/conditions/fake"

	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	c := NewController(ctx, configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sugar.ConfigName,
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
