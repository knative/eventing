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
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

type Reconciler struct {
	authResolver *resolver.AuthenticatableResolver
}

// ReconcileKind implements Interface.ReconcileKind.
// 1. Verify the Reference exists.
func (r *Reconciler) ReconcileKind(ctx context.Context, ep *v1alpha1.EventPolicy) pkgreconciler.Event {
	featureFlags := feature.FromContext(ctx)
	if featureFlags.IsOIDCAuthentication() {
		ep.Status.MarkOIDCAuthenticationEnabled()
	} else {
		ep.Status.MarkOIDCAuthenticationNotEnabled("AuthOIDCFeatureNotEnabled", "")
		return nil
	}
	// We reconcile the status of the EventPolicy
	// by looking at all .spec.from[].refs have subjects
	// and accordingly set the eventpolicy status
	subjects, err := auth.ResolveSubjects(r.authResolver, ep)
	if err != nil {
		ep.Status.MarkSubjectsNotResolved("SubjectsNotResolved", err.Error())
		return fmt.Errorf("failed to resolve .spec.from[].ref: %w", err)
	}
	ep.Status.MarkSubjectsResolved()
	ep.Status.From = subjects
	return nil
}
