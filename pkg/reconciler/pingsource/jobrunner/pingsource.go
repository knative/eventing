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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/kubernetes/kube-openapi/pkg/util/sets"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	pkgreconciler "knative.dev/pkg/reconciler"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	sourceslisters "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"
	"knative.dev/eventing/pkg/logging"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	sourceslisters "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"
	"knative.dev/eventing/pkg/logging"
)

const (
	finalizerName = "jobrunners.pingsources.sources.knative.dev"
)

// Reconciler reconciles PingSources
type Reconciler struct {
	cronRunner       *cronJobsRunner
	pingsourceLister sourceslisters.PingSourceLister

	entryidMu sync.Mutex
	entryids  map[string]cron.EntryID // key: resource namespace/name
}

// Check that our Reconciler implements ReconcileKind.
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

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
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha2.PingSource) error {
	key := fmt.Sprintf("%s/%s", source.Namespace, source.Name)
	if !source.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, key, source)
	}

	// Make sure finalizer is set
	finalizers := sets.NewString(source.Finalizers...)
	if !finalizers.Has(finalizerName) {
		finalizers.Insert(finalizerName)
		var err error
		if source, err = r.patchFinalizer(source, finalizers.List()); err != nil {
			logging.FromContext(ctx).Debug("Failed to update finalizer", zap.Error(err))
			return err
		}
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

func (r *Reconciler) finalize(ctx context.Context, key string, source *v1alpha2.PingSource) error {
	if id, ok := r.entryids[key]; ok {
		r.cronRunner.RemoveSchedule(id)

		r.entryidMu.Lock()
		delete(r.entryids, key)
		r.entryidMu.Unlock()
	}

	// Now we can remove the finalizer
	finalizers := sets.NewString(source.Finalizers...)
	if finalizers.Has(finalizerName) {
		finalizers.Delete(finalizerName)
		if _, err := r.patchFinalizer(source, finalizers.List()); err != nil {
			logging.FromContext(ctx).Debug("Failed to remove finalizer", zap.Error(err))
			return err
		}
	}

	return nil
}

func (r *Reconciler) patchFinalizer(source *v1alpha2.PingSource, finalizers []string) (*v1alpha2.PingSource, error) {
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      finalizers,
			"resourceVersion": source.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return source, err
	}

	source, err = r.EventingClientSet.SourcesV1alpha2().PingSources(source.Namespace).Patch(source.Name, types.MergePatchType, patch)
	if err != nil {
		r.Recorder.Eventf(source, v1.EventTypeWarning, "FinalizerUpdateFailed",
			"Failed to update finalizers for %q: %v", source.Name, err)
	} else {
		r.Recorder.Eventf(source, v1.EventTypeNormal, "FinalizerUpdate",
			"Updated %q finalizers", source.GetName())
	}
	return source, err
}
