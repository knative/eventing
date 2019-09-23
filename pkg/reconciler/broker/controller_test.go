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

package broker

import (
	"os"
	"testing"

	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/subscription/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/service/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	_ = os.Setenv("BROKER_INGRESS_IMAGE", "INGRESS_IMAGE")
	_ = os.Setenv("BROKER_INGRESS_SERVICE_ACCOUNT", "INGRESS_SERVICE_ACCOUNT")
	_ = os.Setenv("BROKER_FILTER_IMAGE", "FILTER_IMAGE")
	_ = os.Setenv("BROKER_FILTER_SERVICE_ACCOUNT", "FILTER_SERVICE_ACCOUNT")

	c := NewController(ctx, configmap.NewStaticWatcher())

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
