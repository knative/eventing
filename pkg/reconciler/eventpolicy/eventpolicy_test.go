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
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventpolicy"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	duckv1authstatus "knative.dev/pkg/client/injection/ducks/duck/v1/authstatus"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
)

const (
	testNS              = "test-namespace"
	eventPolicyName     = "test-eventpolicy"
	pingSourceName      = "test-pingsource"
	apiServerSourceName = "test-apiserversource"
	serviceAccountname  = "test-sa"
)

var (
	pingSourceWithServiceAccount      = NewPingSource(pingSourceName, testNS, WithPingSourceOIDCServiceAccountName(serviceAccountname))
	apiServerSourceWithServiceAccount = NewApiServerSource(apiServerSourceName, testNS, WithApiServerSourceOIDCServiceAccountName((serviceAccountname)))
)

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		},
		{
			Name: "subject not found, status set to NotReady",
			Key:  testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS),
						WithUnreadyEventPolicyCondition),
				},
			},
			WantErr: false,
		},
		{
			Name: "subject found for pingsource, status set to Ready",
			Key:  testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				pingSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithInitEventPolicyConditions,
						WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname)}),
						WithReadyEventPolicyCondition),
				},
			},
			WantErr: false,
		},
		{
			Name: "subject found for apiserversource, status set to Ready",
			Key:  testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				apiServerSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions, WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("APIServerSource")), apiServerSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("APIServerSource")), apiServerSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname)}),
						WithReadyEventPolicyCondition),
				},
			},
			WantErr: false,
		},
		{
			Name: "Multiple subjects found, status set to Ready",
			Key:  testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				apiServerSourceWithServiceAccount,
				pingSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS),
					WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("APIServerSource")), apiServerSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource")), pingSourceName, testNS),
						WithEventPolicyFrom(v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("APIServerSource")), apiServerSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{
							fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname),
							fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname),
						}),
						WithReadyEventPolicyCondition),
				},
			},
			WantErr: false,
		},
	}
	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = duckv1authstatus.WithDuck(ctx)
		r := &Reconciler{
			fromRefResolver: resolver.NewAuthenticatableResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0))}
		return eventpolicy.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetEventPolicyLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}
