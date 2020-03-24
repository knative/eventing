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
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	sourceslisters "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"

	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/controller"
)

// Reconciler reconciles PingSources
type Reconciler struct {
	cronRunner       *cronJobsRunner
	pingsourceLister sourceslisters.PingSourceLister

	entryidMu sync.Mutex
	entryids  map[string]cron.EntryID // key: resource namespace/name
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the PingSource resource with this namespace/name.
	source, err := r.pingsourceLister.PingSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("PingSource key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	scope, ok := source.Annotations[eventing.ScopeAnnotationKey]
	if ok && scope != eventing.ScopeCluster {
		// Not our responsibility
		logging.FromContext(ctx).Info("Skipping non-cluster-scoped PingSource", zap.Any("key", key))
		return nil
	}

	if !source.Status.IsReady() {
		return fmt.Errorf("PingSource is not ready. Cannot configure the cron jobs runner")
	}

	reconcileErr := r.reconcile(ctx, key, source)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling PingSource", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("PingSource reconciled")
	}
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, key string, source *v1alpha2.PingSource) error {
	if source.DeletionTimestamp != nil {
		if id, ok := r.entryids[key]; ok {
			r.cronRunner.RemoveSchedule(id)

			r.entryidMu.Lock()
			delete(r.entryids, key)
			r.entryidMu.Unlock()
		}
		return nil
	}
	logging.FromContext(ctx).Info("synchronizing schedule")

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
