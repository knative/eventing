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

	corev1 "k8s.io/api/core/v1"

	pkgreconciler "knative.dev/pkg/reconciler"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/reconciler"
)

// newPingSourceSkipped makes a new reconciler event with event type Normal, and
// reason PingSourceNotReady
func newPingSourceSkipped() pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "PingSourceSkipped", "PingSource is not ready")
}

// newPingSourceNotReady makes a new reconciler event with event type Normal, and
// reason PingSourceNotReady
func newPingSourceSynchronized() pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "PingSourceSynchronized", "PingSource adapter is synchronized")
}

// Reconciler reconciles PingSources
type Reconciler struct {
	mtadapter MTAdapter
}

// Check that our Reconciler implements ReconcileKind.
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *sourcesv1.PingSource) reconciler.Event {
	if !source.Status.IsReady() {
		return newPingSourceSkipped()
	}

	// Update the adapter state
	r.mtadapter.Update(ctx, source)

	return newPingSourceSynchronized()
}

func (r *Reconciler) deleteFunc(obj interface{}) {
	if obj == nil {
		return
	}
	acc, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		return
	}
	pingSource, ok := acc.(*sourcesv1.PingSource)
	if !ok || pingSource == nil {
		return
	}
	r.mtadapter.Remove(pingSource)
}
