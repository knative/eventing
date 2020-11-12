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
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"

	"github.com/robfig/cron/v3"

	_ "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/logging"
	rectesting "knative.dev/pkg/reconciler/testing"

	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
)

func TestStartStopAdapter(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	envCfg := NewEnvConfig()

	ce := adaptertest.NewTestClient()
	adapter := NewAdapter(ctx, envCfg, ce)

	done := make(chan struct{})
	go func(ctx context.Context) {
		err := adapter.Start(ctx)
		if err != nil {
			t.Error("Unexpected error:", err)
		}
		close(done)
	}(ctx)

	cancel()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("Expected adapter to be stopped after 2 seconds")
	case <-done:
	}
}

func TestUpdateRemoveAdapter(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	adapter := mtpingAdapter{
		logger:    logging.FromContext(ctx),
		runner:    &testRunner{},
		entryidMu: sync.RWMutex{},
		entryids:  make(map[string]cron.EntryID),
	}

	adapter.Update(ctx, &sourcesv1beta1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	})

	if _, ok := adapter.entryids["test-ns/test-name"]; !ok {
		t.Error(`Expected cron entries to contain "test-ns/test-name"`)
	}

	adapter.Remove(ctx, &sourcesv1beta1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	})
	if _, ok := adapter.entryids["test-ns/test-name"]; ok {
		t.Error(`Expected cron entries to not contain "test-ns/test-name"`)
	}
}

type testRunner struct {
	CronJobRunner
}

func (*testRunner) AddSchedule(*sourcesv1beta1.PingSource) cron.EntryID {
	return cron.EntryID(1)
}
func (*testRunner) RemoveSchedule(cron.EntryID) {}
