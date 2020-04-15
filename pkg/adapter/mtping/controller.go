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

	"knative.dev/eventing/pkg/adapter/v2"

	"github.com/robfig/cron"
	"go.uber.org/zap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha2/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	"knative.dev/eventing/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Setup trace publishing.
	iw := cmw.(*configmap.InformedWatcher)
	if err := tracing.SetupDynamicPublishing(logger, iw, "ping-source-dispatcher", tracingconfig.ConfigName); err != nil {
		logger.Fatalw("Error setting up trace publishing", zap.Error(err))
	}

	pingsourceInformer := pingsourceinformer.Get(ctx)

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		pingsourceLister:  pingsourceInformer.Lister(),
		entryidMu:         sync.RWMutex{},
		entryids:          make(map[string]cron.EntryID),
	}

	impl := pingsourcereconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")

	// Watch for pingsource objects
	pingsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Create the cron job runner
	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	ceClient, err := adapter.NewCloudEventsClient("", nil, reporter)
	if err != nil {
		logger.Fatalw("Error setting up trace publishing", zap.Error(err))
	}

	r.cronRunner = NewCronJobsRunner(ceClient, logger)

	// Start the cron job runner.
	go func() {
		err := r.cronRunner.Start(ctx.Done())
		if err != nil {
			logger.Error("Failed stopping the cron jobs runner.", zap.Error(err))
		}
	}()

	return impl
}
