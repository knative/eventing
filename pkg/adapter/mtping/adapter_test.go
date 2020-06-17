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

package mtping

import (
	"context"
	"testing"
	"time"

	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	rectesting "knative.dev/pkg/reconciler/testing"
)

func TestStartStopAdapter(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	envCfg := NewEnvConfig()
	ce := adaptertest.NewTestClient()
	adapter := NewAdapter(ctx, envCfg, ce)

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan bool)
	go func(ctx context.Context) {
		err := adapter.Start(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		done <- true
	}(ctx)

	cancel()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("expected adapter to be stopped after 2 seconds")
	case <-done:
	}
}
