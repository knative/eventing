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

package dispatcher

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/eventingtls"

	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"

	configmap "knative.dev/pkg/configmap/informer"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection client
	_ "knative.dev/eventing/pkg/client/injection/client/fake"
	// Fake injection informers
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
	_ "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta2/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel/fake"
)

func TestNew(t *testing.T) {
	ctx, cancel, _ := SetupFakeContextWithCancel(t, SetUpInformerSelector)

	defer cancel()
	// Replace test logger because the shutdown of the dispatcher may happen
	// after the test ends, causing a data race on the t logger
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())

	os.Setenv("SCOPE", eventing.ScopeCluster)
	os.Setenv("POD_NAME", "testpod")
	os.Setenv("CONTAINER_NAME", "testcontainer")
	os.Setenv("MAX_IDLE_CONNS", "2000")
	os.Setenv("MAX_IDLE_CONNS_PER_HOST", "200")
	c := NewController(ctx, &configmap.InformedWatcher{})

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func TestNewInNamespace(t *testing.T) {
	ctx, cancel, _ := SetupFakeContextWithCancel(t, SetUpInformerSelector)
	defer cancel()
	// Replace test logger because the shutdown of the dispatcher may happen
	// after the test ends, causing a data race on the t logger
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())

	os.Setenv("SCOPE", eventing.ScopeNamespace)
	os.Setenv("POD_NAME", "testpod")
	os.Setenv("CONTAINER_NAME", "testcontainer")
	os.Setenv("MAX_IDLE_CONNS", "2000")
	os.Setenv("MAX_IDLE_CONNS_PER_HOST", "200")
	c := NewController(ctx, &configmap.InformedWatcher{})

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func TestMaxIdleConnsEqualToZero(t *testing.T) {
	ctx, cancel, _ := SetupFakeContextWithCancel(t, SetUpInformerSelector)
	defer cancel()
	// Replace test logger because the shutdown of the dispatcher may happen
	// after the test ends, causing a data race on the t logger
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())

	os.Setenv("SCOPE", eventing.ScopeNamespace)
	os.Setenv("POD_NAME", "testpod")
	os.Setenv("CONTAINER_NAME", "testcontainer")
	os.Setenv("MAX_IDLE_CONNS", "0")
	os.Setenv("MAX_IDLE_CONNS_PER_HOST", "200")

	require.Panics(t, func() {
		NewController(ctx, &configmap.InformedWatcher{})
	})
}

func TestMaxIdleConnsPerHostEqualToZero(t *testing.T) {
	ctx, cancel, _ := SetupFakeContextWithCancel(t, SetUpInformerSelector)
	defer cancel()
	// Replace test logger because the shutdown of the dispatcher may happen
	// after the test ends, causing a data race on the t logger
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())

	os.Setenv("SCOPE", eventing.ScopeNamespace)
	os.Setenv("POD_NAME", "testpod")
	os.Setenv("CONTAINER_NAME", "testcontainer")
	os.Setenv("MAX_IDLE_CONNS", "2000")
	os.Setenv("MAX_IDLE_CONNS_PER_HOST", "0")

	require.Panics(t, func() {
		NewController(ctx, &configmap.InformedWatcher{})
	})
}

func SetUpInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, eventingtls.TrustBundleLabelSelector)
	return ctx
}
