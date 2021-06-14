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
	"testing"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/apiserversource"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
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

	rttesting "knative.dev/eventing/pkg/reconciler/testing"
	rttestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/pkg/reconciler/testing"
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
				rttestingv1.WithChannelAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Insufficient permission: user system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group ""`),
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
				rttestingv1.WithChannelAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
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
				rttestingv1.WithChannelAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
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
				rttestingv1.WithChannelAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
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
			Eventf(corev1.EventTypeWarning, "SinkNotFound",
				`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1"}}`),
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
				rttestingv1.WithChannelAddress(sinkDNS),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, apiserversourceDeploymentCreated,
				"Deployment created, error:inducing failure for create deployments"),
			Eventf(corev1.EventTypeWarning, "InternalError",
				"inducing failure for create deployments"),
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
				rttestingv1.WithChannelAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
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
				rttestingv1.WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentEnv(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
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
				rttestingv1.WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentServiceAccount(t, "morgan"),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
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
				rttestingv1.WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentContainerCount(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", `Deployment "apiserversource-test-apiserver-source-1234" updated`),
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
				rttestingv1.WithBrokerAddress(sinkDNS),
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
			),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, rttestingv1.MakeFactory(func(ctx context.Context, listers *rttestingv1.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:       fakekubeclient.Get(ctx),
			ceSource:            source,
			receiveAdapterImage: image,
			sinkResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			configs:             &reconcilersource.EmptyVarsGenerator{},
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
		Image:   image,
		Source:  src,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI.String(),
		Configs: &reconcilersource.EmptyVarsGenerator{},
	}

	ra, err := resources.MakeReceiveAdapter(&args)
	require.NoError(t, err)

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
		Image:   image,
		Source:  src,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkTargetURI.String(),
		Configs: &reconcilersource.EmptyVarsGenerator{},
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
		Image:   image,
		Source:  src,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI.String(),
		Configs: &reconcilersource.EmptyVarsGenerator{},
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

func makeSubjectAccessReview(resource, verb, sa string) *authorizationv1.SubjectAccessReview {
	return &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: testNS,
				Verb:      verb,
				Group:     "",
				Resource:  resource,
			},
			User: "system:serviceaccount:" + testNS + ":" + sa,
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
