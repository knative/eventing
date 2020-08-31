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
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta1/apiserversource"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	rttesting "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1beta1",
		},
	}
	brokerDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1beta1",
		},
	}
	sinkDNS          = "sink.mynamespace.svc." + utils.GetClusterDomainName()
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

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	table := TableTest{{
		Name: "not enough permissions",
		Objects: []runtime.Object{
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceNoSufficientPermissionsV1B1,
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeployedV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeployedV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
		Name: "valid with relative uri reference",
		Objects: []runtime.Object{
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
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
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapterWithTargetURI(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
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
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeployedV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkTargetURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentEnv(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceDeploymentUnavailableV1B1,
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: sinkDest,
					},
					ServiceAccountName: "malin",
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentServiceAccount(t, "morgan"),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{
						Sink: sinkDest,
					},
					ServiceAccountName: "malin",
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeploymentUnavailableV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentContainerCount(t),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeploymentUnavailableV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
			rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: brokerDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
			),
			NewBroker(sinkName, testNS,
				WithInitBrokerConditions,
				WithBrokerAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(t),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
				rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
					Resources: []sourcesv1beta1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Namespace",
					}},
					SourceSpec: duckv1.SourceSpec{Sink: brokerDest},
				}),
				rttesting.WithApiServerSourceUIDV1B1(sourceUID),
				rttesting.WithApiServerSourceObjectMetaGenerationV1B1(generation),
				// Status Update:
				rttesting.WithInitApiServerSourceConditionsV1B1,
				rttesting.WithApiServerSourceDeployedV1B1,
				rttesting.WithApiServerSourceSinkV1B1(sinkURI),
				rttesting.WithApiServerSourceSufficientPermissionsV1B1,
				rttesting.WithApiServerSourceEventTypesV1B1(source),
				rttesting.WithApiServerSourceStatusObservedGenerationV1B1(generation),
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
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:       fakekubeclient.Get(ctx),
			ceSource:            source,
			receiveAdapterImage: image,
			sinkResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			configs:             &reconcilersource.EmptyVarsGenerator{},
		}
		return apiserversource.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetApiServerSourceV1beta1Lister(),
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

	src := rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
		rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
			Resources: []sourcesv1beta1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttesting.WithApiServerSourceUIDV1B1(sourceUID),
		// Status Update:
		rttesting.WithInitApiServerSourceConditionsV1B1,
		rttesting.WithApiServerSourceDeployedV1B1,
		rttesting.WithApiServerSourceSinkV1B1(sinkURI),
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

	src := rttesting.NewApiServerSourceV1Beta1(sourceName, testNS,
		rttesting.WithApiServerSourceSpecV1B1(sourcesv1beta1.ApiServerSourceSpec{
			Resources: []sourcesv1beta1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Namespace",
			}},
			SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
		}),
		rttesting.WithApiServerSourceUIDV1B1(sourceUID),
		// Status Update:
		rttesting.WithInitApiServerSourceConditionsV1B1,
		rttesting.WithApiServerSourceDeployedV1B1,
		rttesting.WithApiServerSourceSinkV1B1(sinkURI),
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
