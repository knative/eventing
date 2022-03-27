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

package adapter

import (
	"context"
	"testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	_ "knative.dev/pkg/client/injection/kube/client/fake"
)

func TestWithController(t *testing.T) {
	ctx := WithController(context.TODO(), func(ctx context.Context, adapter Adapter) *controller.Impl {
		return nil
	})

	if ControllerFromContext(ctx) == nil {
		t.Error("expected non-nil controller constructor")
	}
}

func TestWithHAEnabled(t *testing.T) {
	ctx := context.Background()
	ctx = WithHAEnabled(ctx)
	if !IsHAEnabled(ctx) {
		t.Error("Expected HA to be enabled")
	}

	ctx = withHADisabledFlag(ctx)
	if !IsHAEnabled(ctx) {
		t.Error("Expected HA to be enabled")
	}
	if !isHADisabledFlag(ctx) {
		t.Error("Expected HA to be disabled via commandline flag")
	}
}

func TestWithConfigWatcher(t *testing.T) {
	ctx := context.Background()

	if IsConfigWatcherEnabled(ctx) {
		t.Error("Expected ConfigWatcher not enabled default")
	}

	ctx = WithConfigWatcherEnabled(ctx)
	if !IsConfigWatcherEnabled(ctx) {
		t.Error("Expected ConfigWatcher to be enabled")
	}

	cmw := ConfigWatcherFromContext(ctx)
	if cmw != nil {
		t.Error("Expected ConfigWatcher at context to be nil")
	}

	mw := &configmap.ManualWatcher{}
	ctx = WithConfigWatcher(ctx, mw)

	if cmw = ConfigWatcherFromContext(ctx); cmw != mw {
		t.Error("Expected ConfigWatcher to match the one set previously")
	}
}
