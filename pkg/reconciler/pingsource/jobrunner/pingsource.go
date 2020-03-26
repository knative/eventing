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

package jobrunner

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron"
	"go.uber.org/zap"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	sourceslisters "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"
	"knative.dev/eventing/pkg/logging"
)

// Reconciler reconciles PingSources
type Reconciler struct {
	cronRunner        *cronJobsRunner
	eventingClientSet clientset.Interface
	pingsourceLister  sourceslisters.PingSourceLister

	entryidMu sync.Mutex
	entryids  map[string]cron.EntryID // key: resource namespace/name
}

// Check that our Reconciler implements ReconcileKind.
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

// Check that our Reconciler implements FinalizeKind.
var _ pingsourcereconciler.Finalizer = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha2.PingSource) pkgreconciler.Event {
	scope, ok := source.Annotations[eventing.ScopeAnnotationKey]
	if ok && scope != eventing.ScopeCluster {
		// Not our responsibility
		logging.FromContext(ctx).Info("Skipping non-cluster-scoped PingSource", zap.Any("namespace", source.Namespace), zap.Any("name", source.Name))
		return nil
	}

	if !source.Status.IsReady() {
		return fmt.Errorf("PingSource is not ready. Cannot configure the cron jobs runner")
	}

	reconcileErr := r.reconcile(ctx, source)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling PingSource", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("PingSource reconciled")
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha2.PingSource) error {
	logging.FromContext(ctx).Info("synchronizing schedule")

	key := fmt.Sprintf("%s/%s", source.Namespace, source.Name)
	// Is the schedule already cached?
	if id, ok := r.entryids[key]; ok {
		r.cronRunner.RemoveSchedule(id)
	}

	// The schedule has already been validated by the validation webhook, so ignoring error
	id, _ := r.cronRunner.AddSchedule(source.Namespace, source.Name, source.Spec.Schedule, source.Spec.JsonData, source.Status.SinkURI.String())

	r.entryidMu.Lock()
	r.entryids[key] = id
	r.entryidMu.Unlock()

	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *v1alpha2.PingSource) pkgreconciler.Event {
	key := fmt.Sprintf("%s/%s", source.Namespace, source.Name)

	if id, ok := r.entryids[key]; ok {
		r.cronRunner.RemoveSchedule(id)

		r.entryidMu.Lock()
		delete(r.entryids, key)
		r.entryidMu.Unlock()
	}

	return nil
}
