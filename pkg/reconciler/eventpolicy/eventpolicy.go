/*
Copyright 2024 The Knative Authors

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

package eventpolicy

import (
	"context"

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

type Reconciler struct {
	eventPolicyLister eventinglisters.EventPolicyLister
	fromRefResolver   *resolver.AuthenticatableResolver
}

// ReconcileKind implements Interface.ReconcileKind.
// 1. Verify the Reference exists.
func (r *Reconciler) ReconcileKind(ctx context.Context, ep *v1alpha1.EventPolicy) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infow("Reconciling", zap.Any("EventPolicy", ep))
	// We reconcile the status of the EventPolicy by looking at:
	// 1. All from[].refs have subjects
	serverAccts, err := auth.ResolveSubjects(r.fromRefResolver, ep)
	if err != nil {
		logger.Errorw("Error resolving from[].refs", zap.Error(err))
		ep.GetConditionSet().Manage(ep.GetStatus()).MarkFalse(v1alpha1.EventPolicyConditionReady, "Error resolving from[].refs", "")
	} else {
		logger.Debug("All from[].refs resolved", zap.Error(err))
		ep.GetConditionSet().Manage(ep.GetStatus()).MarkTrue(v1alpha1.EventPolicyConditionReady)
	}
	ep.Status.From = serverAccts
	logger.Debugw("Reconciled EventPolicy", zap.Any("EventPolicy", ep))
	return nil
}
