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

	"github.com/robfig/cron/v3"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha2/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(ctx context.Context, cronRunner *cronJobsRunner) *controller.Impl {
	logger := logging.FromContext(ctx)

	pingsourceInformer := pingsourceinformer.Get(ctx)

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		pingsourceLister:  pingsourceInformer.Lister(),
		entryidMu:         sync.RWMutex{},
		entryids:          make(map[string]cron.EntryID),
		cronRunner:        cronRunner,
	}

	impl := pingsourcereconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")

	// Watch for pingsource objects
	pingsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
