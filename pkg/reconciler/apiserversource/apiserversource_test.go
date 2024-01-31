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

package apiserversource

import (
	"context"
	"fmt"
	"testing"

	"knative.dev/pkg/kmeta"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing/pkg/apis/feature"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/auth"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/apiserversource"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	. "knative.dev/pkg/reconciler/testing"

	rttesting "knative.dev/eventing/pkg/reconciler/testing"
	rttestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"

	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/role/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/fake"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}
	brokerDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
		},
	}
	sinkDNS          = "sink.mynamespace.svc." + network.GetClusterDomainName()
	sinkURI          = apis.HTTP(sinkDNS)
	sinkURIReference = "/foo"
	sinkTargetURI    = func() *apis.URL {
		u := apis.HTTP(sinkDNS)
		u.Path = sinkURIReference
		return u
	}()
	sinkURL         = apis.HTTP(sinkDNS)
	sinkAddressable = &duckv1.Addressable{
		Name: &sinkURL.Scheme,
		URL:  sinkURL,
	}

	sinkAudience        = "sink-oidc-audience"
	sinkOIDCAddressable = &duckv1.Addressable{
		Name:     &sinkURL.Scheme,
		URL:      sinkURL,
		Audience: &sinkAudience,
	}
	sinkOIDCDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
		Audience: &sinkAudience,
	}
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-apiserver-source"
	sourceUID  = "1234"
	testNS     = "testnamespace"

	sinkName = "testsink"
	source   = "apiserveraddr"

	generation = 1
)

func TestReconcile(t *testing.T) {
	table := TableTest{{
		Name: "not enough permissions",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceNoSufficientPermissions,
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InternalError", `insufficient permissions: User system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group "" in Namespace "testnamespace"`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(false)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with namespace selector",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				rttestingv1.WithApiServerSourceNamespaceSelector(metav1.LabelSelector{MatchLabels: map[string]string{"target": "yes"}}),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapter(t),
			rttesting.NewNamespace("test-a", rttesting.WithNamespaceLabeled(map[string]string{"target": "yes"})),
			rttesting.NewNamespace("test-b", rttesting.WithNamespaceLabeled(map[string]string{"target": "yes"})),
			rttesting.NewNamespace("test-c", rttesting.WithNamespaceLabeled(map[string]string{"target": "no"})),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceNamespaceSelector(metav1.LabelSelector{MatchLabels: map[string]string{"target": "yes"}}),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{"test-a", "test-b"}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeNamespacedSubjectAccessReview("namespaces", "get", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "list", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "watch", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "get", "default", "test-b"),
			makeNamespacedSubjectAccessReview("namespaces", "list", "default", "test-b"),
			makeNamespacedSubjectAccessReview("namespaces", "watch", "default", "test-b"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeAvailableReceiveAdapterWithNamespaces(t, []string{"test-a", "test-b"}, false),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with an empty namespace selector",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				rttestingv1.WithApiServerSourceNamespaceSelector(metav1.LabelSelector{}),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapter(t),
			rttesting.NewNamespace("test-a"),
			rttesting.NewNamespace("test-b"),
			rttesting.NewNamespace("test-c"),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceNamespaceSelector(metav1.LabelSelector{}),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{"test-a", "test-b", "test-c"}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeNamespacedSubjectAccessReview("namespaces", "get", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "list", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "watch", "default", "test-a"),
			makeNamespacedSubjectAccessReview("namespaces", "get", "default", "test-b"),
			makeNamespacedSubjectAccessReview("namespaces", "list", "default", "test-b"),
			makeNamespacedSubjectAccessReview("namespaces", "watch", "default", "test-b"),
			makeNamespacedSubjectAccessReview("namespaces", "get", "default", "test-c"),
			makeNamespacedSubjectAccessReview("namespaces", "list", "default", "test-c"),
			makeNamespacedSubjectAccessReview("namespaces", "watch", "default", "test-c"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeAvailableReceiveAdapterWithNamespaces(t, []string{"test-a", "test-b", "test-c"}, true),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with eventmode of resourcemode",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					EventMode:  sourcesv1.ResourceMode,
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapterWithEventMode(t, sourcesv1.ResourceMode),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					EventMode:  sourcesv1.ResourceMode,
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceResourceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with sink URI",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "missing sink",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SinkNotFound",
				`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1"}}`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceSinkNotFound,
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "receive adapter does not exist, fails to create",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, apiserversourceDeploymentCreated,
				"Deployment created, error:inducing failure for create deployments"),
			Eventf(corev1.EventTypeWarning, "InternalError",
				"inducing failure for create deployments"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
			makeReceiveAdapter(t),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			subjectAccessReviewCreateReactor(true),
			InduceFailure("create", "Deployments"),
		},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.

	}, {
		Name: "valid with relative uri reference",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: sinkDest.Ref,
							URI: &apis.URL{Path: sinkURIReference},
						},
					},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeAvailableReceiveAdapterWithTargetURI(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: sinkDest.Ref,
							URI: &apis.URL{Path: sinkURIReference},
						},
					},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkTargetURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "deployment update due to env",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeReceiveAdapterWithDifferentEnv(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceDeploymentUnavailable,
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapter(t),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "deployment update due to service account",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: sinkDest,
					},
					ServiceAccountName: "malin",
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeReceiveAdapterWithDifferentServiceAccount(t, "morgan"),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: sinkDest,
					},
					ServiceAccountName: "malin",
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeploymentUnavailable,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapterWithDifferentServiceAccount(t, "malin"),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "malin"),
			makeSubjectAccessReview("namespaces", "list", "malin"),
			makeSubjectAccessReview("namespaces", "watch", "malin"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "deployment update due to container count",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewChannel(sinkName, testNS,
				rttestingv1.WithInitChannelConditions,
				rttestingv1.WithChannelAddress(sinkAddressable),
			),
			makeReceiveAdapterWithDifferentContainerCount(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeploymentUnavailable,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapter(t),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with broker sink",
		Objects: []runtime.Object{
			rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: brokerDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
			),
			rttestingv1.NewBroker(sinkName, testNS,
				rttestingv1.WithInitBrokerConditions,
				rttestingv1.WithBrokerAddressURI(apis.HTTP(sinkDNS)),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttestingv1.NewApiServerSource(sourceName, testNS,
				rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
					Resources: []sourcesv1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: brokerDest},
				}),
				rttestingv1.WithApiServerSourceUID(sourceUID),
				rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				rttestingv1.WithInitApiServerSourceConditions,
				rttestingv1.WithApiServerSourceDeployed,
				rttestingv1.WithApiServerSourceSink(sinkURI),
				rttestingv1.WithApiServerSourceSufficientPermissions,
				rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
				rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
				rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
				rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(sourceName, testNS),
		},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	},
		{
			Name: "OIDC: creates OIDC service account",
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkOIDCDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				),
				rttestingv1.NewChannel(sinkName, testNS,
					rttestingv1.WithInitChannelConditions,
					rttestingv1.WithChannelAddress(sinkOIDCAddressable),
				),
				makeOIDCRole(),
				makeOIDCRoleBinding(),
				makeAvailableReceiveAdapterWithOIDC(t),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkOIDCDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
					// Status Update:
					rttestingv1.WithInitApiServerSourceConditions,
					rttestingv1.WithApiServerSourceDeployed,
					rttestingv1.WithApiServerSourceSinkAddressable(sinkOIDCAddressable),
					rttestingv1.WithApiServerSourceSufficientPermissions,
					rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
					rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
					rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
					rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceeded(),
					rttestingv1.WithApiServerSourceOIDCServiceAccountName(makeApiServerSourceOIDCServiceAccount().Name),
				),
			}},
			WantCreates: []runtime.Object{
				makeApiServerSourceOIDCServiceAccount(),
				makeSubjectAccessReview("namespaces", "get", "default"),
				makeSubjectAccessReview("namespaces", "list", "default"),
				makeSubjectAccessReview("namespaces", "watch", "default"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
			SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
		},
		{
			Name: "OIDC: ApiServerSource not ready on invalid OIDC service account",
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				makeApiServerSourceOIDCServiceAccountWithoutOwnerRef(),
				rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				),
				rttestingv1.NewChannel(sinkName, testNS,
					rttestingv1.WithInitChannelConditions,
					rttestingv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableReceiveAdapter(t),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
					// Status Update:
					rttestingv1.WithInitApiServerSourceConditions,
					rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
					rttestingv1.WithApiServerSourceOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", fmt.Sprintf("service account %s not owned by ApiServerSource %s", makeApiServerSourceOIDCServiceAccountWithoutOwnerRef().Name, sourceName)),
					rttestingv1.WithApiServerSourceOIDCServiceAccountName(makeApiServerSourceOIDCServiceAccount().Name),
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("service account %s not owned by ApiServerSource %s", makeApiServerSourceOIDCServiceAccountWithoutOwnerRef().Name, sourceName)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
			SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
		},
		{
			Name: "OIDC: creates role and rolebinding to create OIDC token",
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkOIDCDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				),
				rttestingv1.NewChannel(sinkName, testNS,
					rttestingv1.WithInitChannelConditions,
					rttestingv1.WithChannelAddress(sinkOIDCAddressable),
				),
				makeAvailableReceiveAdapterWithOIDC(t),
				makeApiServerSourceOIDCServiceAccount(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkOIDCDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
					// Status Update:
					rttestingv1.WithInitApiServerSourceConditions,
					rttestingv1.WithApiServerSourceDeployed,
					rttestingv1.WithApiServerSourceSinkAddressable(sinkOIDCAddressable),
					rttestingv1.WithApiServerSourceSufficientPermissions,
					rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
					rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
					rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
					rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceeded(),
					rttestingv1.WithApiServerSourceOIDCServiceAccountName(makeApiServerSourceOIDCServiceAccount().Name),
				),
			}},
			WantCreates: []runtime.Object{
				makeOIDCRole(),
				makeOIDCRoleBinding(),
				makeSubjectAccessReview("namespaces", "get", "default"),
				makeSubjectAccessReview("namespaces", "list", "default"),
				makeSubjectAccessReview("namespaces", "watch", "default"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
			SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
		}, {
			Name: "Valid with nodeSelector",

			Ctx: feature.ToContext(context.Background(), feature.Flags{
				"apiserversources.nodeselector.testkey1": "testvalue1",
				"apiserversources.nodeselector.testkey2": "testvalue2",
			}),
			Objects: []runtime.Object{
				rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
				),
				rttestingv1.NewChannel(sinkName, testNS,
					rttestingv1.WithInitChannelConditions,
					rttestingv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableReceiveAdapter(t),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rttestingv1.NewApiServerSource(sourceName, testNS,
					rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
						Resources: []sourcesv1.APIVersionKindSelector{{
							APIVersion: "v1",
							Kind:       "Namespace",
						}},
						SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
					}),
					rttestingv1.WithApiServerSourceUID(sourceUID),
					rttestingv1.WithApiServerSourceObjectMetaGeneration(generation),
					// Status Update:
					rttestingv1.WithInitApiServerSourceConditions,
					rttestingv1.WithApiServerSourceDeployed,
					rttestingv1.WithApiServerSourceSink(sinkURI),
					rttestingv1.WithApiServerSourceSufficientPermissions,
					rttestingv1.WithApiServerSourceReferenceModeEventTypes(source),
					rttestingv1.WithApiServerSourceStatusObservedGeneration(generation),
					rttestingv1.WithApiServerSourceStatusNamespaces([]string{testNS}),
					rttestingv1.WithApiServerSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeSubjectAccessReview("namespaces", "get", "default"),
				makeSubjectAccessReview("namespaces", "list", "default"),
				makeSubjectAccessReview("namespaces", "watch", "default"),
			},

			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: makeAvailableReceiveAdapterWithNodeSelector(t, map[string]string{
					"testkey1": "testvalue1",
					"testkey2": "testvalue2",
				}),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
			SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, rttestingv1.MakeFactory(func(ctx context.Context, listers *rttestingv1.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:              fakekubeclient.Get(ctx),
			ceSource:                   source,
			receiveAdapterImage:        image,
			sinkResolver:               resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
			configs:                    &reconcilersource.EmptyVarsGenerator{},
			namespaceLister:            listers.GetNamespaceLister(),
			serviceAccountLister:       listers.GetServiceAccountLister(),
			roleBindingLister:          listers.GetRoleBindingLister(),
			roleLister:                 listers.GetRoleLister(),
			trustBundleConfigMapLister: listers.GetConfigMapLister(),
		}
		return apiserversource.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetApiServerSourceLister(),
			controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeReceiveAdapter(t *testing.T) *appsv1.Deployment {
	return makeReceiveAdapterWithName(t, sourceName)
}

func makeReceiveAdapterWithName(t *testing.T, sourceName string) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:      image,
		Source:     src,
		Labels:     resources.Labels(sourceName),
		SinkURI:    sinkURI.String(),
		Configs:    &reconcilersource.EmptyVarsGenerator{},
		Namespaces: []string{testNS},
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	return ra
}

func makeReceiveAdapterWithOIDC(t *testing.T) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: sinkOIDCDest,
			},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
		rttestingv1.WithApiServerSourceOIDCServiceAccountName(makeApiServerSourceOIDCServiceAccount().Name),
	)

	args := resources.ReceiveAdapterArgs{
		Image:      image,
		Source:     src,
		Labels:     resources.Labels(sourceName),
		SinkURI:    sinkURI.String(),
		Configs:    &reconcilersource.EmptyVarsGenerator{},
		Namespaces: []string{testNS},
		Audience:   &sinkAudience,
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	return ra
}

func makeAvailableReceiveAdapterWithOIDC(t *testing.T) *appsv1.Deployment {
	ra := makeReceiveAdapterWithOIDC(t)
	rttesting.WithDeploymentAvailable()(ra)

	return ra
}

func makeAvailableReceiveAdapter(t *testing.T) *appsv1.Deployment {
	ra := makeReceiveAdapter(t)
	rttesting.WithDeploymentAvailable()(ra)
	return ra
}

func makeAvailableReceiveAdapterWithTargetURI(t *testing.T) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:      image,
		Source:     src,
		Labels:     resources.Labels(sourceName),
		SinkURI:    sinkTargetURI.String(),
		Configs:    &reconcilersource.EmptyVarsGenerator{},
		Namespaces: []string{testNS},
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	rttesting.WithDeploymentAvailable()(ra)
	return ra
}

func makeAvailableReceiveAdapterWithEventMode(t *testing.T, eventMode string) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			EventMode:  eventMode,
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:      image,
		Source:     src,
		Labels:     resources.Labels(sourceName),
		SinkURI:    sinkURI.String(),
		Configs:    &reconcilersource.EmptyVarsGenerator{},
		Namespaces: []string{testNS},
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	rttesting.WithDeploymentAvailable()(ra)
	return ra
}

func makeAvailableReceiveAdapterWithNamespaces(t *testing.T, namespaces []string, allNamespaces bool) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:         image,
		Source:        src,
		Labels:        resources.Labels(sourceName),
		SinkURI:       sinkURI.String(),
		Configs:       &reconcilersource.EmptyVarsGenerator{},
		Namespaces:    namespaces,
		AllNamespaces: allNamespaces,
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	rttesting.WithDeploymentAvailable()(ra)
	return ra
}

func makeReceiveAdapterWithDifferentEnv(t *testing.T) *appsv1.Deployment {
	ra := makeReceiveAdapter(t)
	ra.Spec.Template.Spec.Containers[0].Env = append(ra.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "not-in",
		Value: "the-original",
	})
	return ra
}

func makeReceiveAdapterWithDifferentServiceAccount(t *testing.T, name string) *appsv1.Deployment {
	ra := makeReceiveAdapter(t)
	ra.Spec.Template.Spec.ServiceAccountName = name
	return ra
}

func makeReceiveAdapterWithDifferentContainerCount(t *testing.T) *appsv1.Deployment {
	ra := makeReceiveAdapter(t)
	ra.Spec.Template.Spec.Containers = append(ra.Spec.Template.Spec.Containers, corev1.Container{})
	return ra
}

func makeNamespacedSubjectAccessReview(resource, verb, sa, ns string) *authorizationv1.SubjectAccessReview {
	return &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: ns,
				Verb:      verb,
				Group:     "",
				Resource:  resource,
			},
			User: "system:serviceaccount:" + testNS + ":" + sa,
		},
	}
}

func makeSubjectAccessReview(resource, verb, sa string) *authorizationv1.SubjectAccessReview {
	return makeNamespacedSubjectAccessReview(resource, verb, sa, testNS)
}

func makeOIDCRole() *rbacv1.Role {
	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceUID(sourceUID),
	)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resources.GetOIDCTokenRoleName(sourceName),
			Namespace: testNS,
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role for OIDC Authentication for ApiServerSource %q", sourceName),
			},
			Labels: map[string]string{
				auth.OIDCLabelKey: "enabled",
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(src),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				// apiServerSource OIDC service account name, it is in the source.Status, NOT in source.Spec
				ResourceNames: []string{makeApiServerSourceOIDCServiceAccount().Name},
				Resources:     []string{"serviceaccounts/token"},
				Verbs:         []string{"create"},
			},
		},
	}
}

func makeOIDCRoleBinding() *rbacv1.RoleBinding {
	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceUID(sourceUID),
	)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resources.GetOIDCTokenRoleBindingName(sourceName),
			Namespace: testNS,
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role Binding for OIDC Authentication for ApiServerSource %q", sourceName),
			},
			Labels: map[string]string{
				auth.OIDCLabelKey: "enabled",
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(src),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     resources.GetOIDCTokenRoleName(sourceName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: testNS,
				Name:      "default",
			},
		},
	}
}

func subjectAccessReviewCreateReactor(allowed bool) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "create" && action.GetResource().Resource == "subjectaccessreviews" {
			ret := action.(clientgotesting.CreateAction).GetObject().DeepCopyObject().(*authorizationv1.SubjectAccessReview)
			ret.Status.Allowed = allowed
			return true, ret, nil
		}
		return false, nil, nil
	}
}

func patchFinalizers(name, namespace string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["apiserversources.sources.knative.dev"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func makeApiServerSourceOIDCServiceAccount() *corev1.ServiceAccount {
	return auth.GetOIDCServiceAccountForResource(sourcesv1.SchemeGroupVersion.WithKind("ApiServerSource"), metav1.ObjectMeta{
		Name:      sourceName,
		Namespace: testNS,
		UID:       sourceUID,
	})
}

func makeApiServerSourceOIDCServiceAccountWithoutOwnerRef() *corev1.ServiceAccount {
	sa := auth.GetOIDCServiceAccountForResource(sourcesv1.SchemeGroupVersion.WithKind("ApiServerSource"), metav1.ObjectMeta{
		Name:      sourceName,
		Namespace: testNS,
		UID:       sourceUID,
	})
	sa.OwnerReferences = nil

	return sa
}

func makeAvailableReceiveAdapterWithNodeSelector(t *testing.T, selector map[string]string) *appsv1.Deployment {
	t.Helper()

	src := rttestingv1.NewApiServerSource(sourceName, testNS,
		rttestingv1.WithApiServerSourceSpec(sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttestingv1.WithApiServerSourceUID(sourceUID),
		// Status Update:
		rttestingv1.WithInitApiServerSourceConditions,
		rttestingv1.WithApiServerSourceDeployed,
		rttestingv1.WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:        image,
		Source:       src,
		Labels:       resources.Labels(sourceName),
		SinkURI:      sinkURI.String(),
		Configs:      &reconcilersource.EmptyVarsGenerator{},
		NodeSelector: selector,
		Namespaces:   []string{testNS},
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

	rttesting.WithDeploymentAvailable()(ra)
	return ra
}
