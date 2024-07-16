/*
Copyright 2019 The Knative Authors

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

package eventtype

import (
	"context"

	"knative.dev/eventing/pkg/resolver"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"knative.dev/eventing/pkg/apis/eventing/v1beta3"
	eventtypereconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta3/eventtype"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	kReferenceResolver *resolver.KReferenceResolver
}

// Check that our Reconciler implements interface
var _ eventtypereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
//  1. Check if there is a reference
//     a) if not, reconcile to true
//     b) if yes, continue reconciling
//  2. Verify the Reference exist
func (r *Reconciler) ReconcileKind(ctx context.Context, et *v1beta3.EventType) pkgreconciler.Event {
	if et.Spec.Reference == nil || isEmptyReference(et.Spec.Reference) {
		et.Status.MarkReferenceNotSet()
		return nil
	}

	_, err := r.kReferenceResolver.Resolve(ctx, et.Spec.Reference, et)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Errorw("Reference does not exist", zap.Error(err))
			et.Status.MarkReferenceDoesNotExist()
		} else {
			logging.FromContext(ctx).Errorw("Unable to get the reference", zap.Error(err))
			et.Status.MarkReferenceExistsUnknown("ReferenceGetFailed", "Failed to get reference: %v", err)
		}
		return err
	}
	et.Status.MarkReferenceExists()
	return nil
}

func isEmptyReference(ref *duckv1.KReference) bool {
	return ref.Kind == "" && ref.Group == "" && ref.Name == "" && ref.APIVersion == "" && ref.Namespace == "" && (ref.Address == nil || *ref.Address == "")
}
