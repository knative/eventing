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
	"knative.dev/eventing/pkg/apis/feature"
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
	pingSourceGVK                     = v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("PingSource"))
	apiServerSourceGVK                = v1.GroupVersionKind(sourcesv1.SchemeGroupVersion.WithKind("APIServerSource"))
	SubjectsNotResolvedErrorMessage   = fmt.Sprintf("could not resolve subjects from reference: could not resolve auth status: failed to get authenticatable %s/%s: failed to get object %s/%s: pingsources.sources.knative.dev \"%s\" not found", testNS, pingSourceName, testNS, pingSourceName, pingSourceName)
	SubjectsNotResolvedEventMessage   = fmt.Sprintf("Warning InternalError failed to resolve .spec.from[].ref: could not resolve subjects from reference: could not resolve auth status: failed to get authenticatable %s/%s: failed to get object %s/%s: pingsources.sources.knative.dev \"%s\" not found", testNS, pingSourceName, testNS, pingSourceName, pingSourceName)
)

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		},
		// test cases for authentication-oidc feature disabled
		{
			Name: "with oidc disabled, status set to NotReady",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Disabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
						WithFalseAuthenticationEnabledCondition,
						WithUnreadyEventPolicyCondition("AuthOIDCFeatureNotEnabled", ""),
						WithUnknownSubjectsResolvedCondition,
					),
				},
			},
			WantErr: false,
		},

		// test cases for authentication-oidc feature enabled
		{
			Name: "subject not found, status set to NotReady",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
						WithTrueAuthenticationEnabledCondition,
						WithUnreadyEventPolicyCondition("SubjectsNotResolved", SubjectsNotResolvedErrorMessage),
						WithFalseSubjectsResolvedCondition("SubjectsNotResolved", SubjectsNotResolvedErrorMessage),
					),
				},
			},
			WantEvents: []string{SubjectsNotResolvedEventMessage},
			WantErr:    true,
		},
		{
			Name: "subject found for pingsource, status set to Ready",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				pingSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname)}),
						WithTrueAuthenticationEnabledCondition,
						WithReadyEventPolicyCondition,
						WithTrueSubjectsResolvedCondition,
					),
				},
			},
			WantErr: false,
		},
		{
			Name: "subject found for apiserversource, status set to Ready",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				apiServerSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions, WithEventPolicyFrom(apiServerSourceGVK, apiServerSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(apiServerSourceGVK, apiServerSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname)}),
						WithTrueAuthenticationEnabledCondition,
						WithReadyEventPolicyCondition,
						WithTrueSubjectsResolvedCondition,
					),
				},
			},
			WantErr: false,
		},
		{
			Name: "Multiple subjects found, status set to Ready",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				apiServerSourceWithServiceAccount,
				pingSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithInitEventPolicyConditions,
					WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
					WithEventPolicyFrom(apiServerSourceGVK, apiServerSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
						WithEventPolicyFrom(apiServerSourceGVK, apiServerSourceName, testNS),
						WithEventPolicyStatusFromSub([]string{
							fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname),
							fmt.Sprintf("system:serviceaccount:%s:%s", testNS, serviceAccountname),
						}),
						WithTrueAuthenticationEnabledCondition,
						WithReadyEventPolicyCondition,
						WithTrueSubjectsResolvedCondition,
					),
				},
			},
			WantErr: false,
		},

		// test cases for authentication-oidc feature disabled afterwards
		{
			Name: "Ready status EventPolicy updated to NotReady",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.OIDCAuthentication: feature.Disabled,
			}),
			Key: testNS + "/" + eventPolicyName,
			Objects: []runtime.Object{
				pingSourceWithServiceAccount,
				NewEventPolicy(eventPolicyName, testNS,
					WithReadyEventPolicyCondition,
					WithTrueAuthenticationEnabledCondition,
					WithTrueSubjectsResolvedCondition,
					WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewEventPolicy(eventPolicyName, testNS,
						WithEventPolicyFrom(pingSourceGVK, pingSourceName, testNS),
						WithFalseAuthenticationEnabledCondition,
						WithUnreadyEventPolicyCondition("AuthOIDCFeatureNotEnabled", ""),
						WithTrueSubjectsResolvedCondition,
					),
				},
			},
			WantErr: false,
		},
	}
	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = duckv1authstatus.WithDuck(ctx)
		r := &Reconciler{
			authResolver: resolver.NewAuthenticatableResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0))}
		return eventpolicy.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetEventPolicyLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}
