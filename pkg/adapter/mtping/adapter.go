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
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
)

const (
	EnvNoShutdownAfter = "K_NO_SHUTDOWN_AFTER"
)

// mtpingAdapter implements the PingSource mt adapter to sinks
type mtpingAdapter struct {
	logger    *zap.SugaredLogger
	runner    CronJobRunner
	entryidMu sync.RWMutex
	entryids  map[string]cron.EntryID // key: resource namespace/name
}

var (
	_ adapter.Adapter = (*mtpingAdapter)(nil)
	_ MTAdapter       = (*mtpingAdapter)(nil)
)

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapter.EnvConfig{}
}

func NewAdapter(ctx context.Context, _ adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	runner := NewCronJobsRunner(ceClient, kubeclient.Get(ctx), logging.FromContext(ctx))

	return &mtpingAdapter{
		logger:    logger,
		runner:    runner,
		entryidMu: sync.RWMutex{},
		entryids:  make(map[string]cron.EntryID),
	}
}

// Start implements adapter.Adapter
func (a *mtpingAdapter) Start(ctx context.Context) error {
	a.logger.Info("Starting job runner...")
	a.runner.Start(ctx.Done())
	defer a.runner.Stop()

	a.logger.Infof("runner stopped")
	return nil
}

func GetNoShutDownAfterValue() int {
	str := os.Getenv(EnvNoShutdownAfter)
	if str != "" {
		second, err := strconv.Atoi(str)
		if err != nil || second < 0 || second > 59 {
			log.Printf("%s environment value is invalid. It must be a integer between 0 and 59. (got %s)", EnvNoShutdownAfter, str)
		} else {
			return second
		}
	}
	return 55 // seems a reasonable default
}

// Implements MTAdapter

func (a *mtpingAdapter) Update(ctx context.Context, source *sourcesv1.PingSource) {
	logging.FromContext(ctx).Info("Synchronizing schedule")

	key := fmt.Sprintf("%s/%s", source.Namespace, source.Name)
	// Is the schedule already cached?
	a.entryidMu.RLock()
	id, ok := a.entryids[key]
	a.entryidMu.RUnlock()

	if ok {
		a.runner.RemoveSchedule(id)
	}

	id = a.runner.AddSchedule(source)

	a.entryidMu.Lock()
	a.entryids[key] = id
	a.entryidMu.Unlock()
}

func (a *mtpingAdapter) Remove(source *sourcesv1.PingSource) {
	key := fmt.Sprintf("%s/%s", source.Namespace, source.Name)

	a.entryidMu.RLock()
	id, ok := a.entryids[key]
	a.entryidMu.RUnlock()

	if ok {
		a.runner.RemoveSchedule(id)

		a.entryidMu.Lock()
		delete(a.entryids, key)
		a.entryidMu.Unlock()
	}
}

func (a *mtpingAdapter) RemoveAll(ctx context.Context) {
	a.entryidMu.Lock()
	defer a.entryidMu.Unlock()

	for _, id := range a.entryids {
		a.runner.RemoveSchedule(id)
	}
	a.entryids = make(map[string]cron.EntryID)
}
