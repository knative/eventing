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

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/apiserversource"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/resolver"

	. "knative.dev/eventing/pkg/reconciler/testing"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkDest = duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	brokerDest = duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}
	sinkDNS          = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI          = "http://" + sinkDNS
	sinkURIReference = "/foo"
	sinkTargetURI    = sinkURI + sinkURIReference
	sinkDestURI      = duckv1beta1.Destination{
		URI: apis.HTTP(sinkDNS),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceStatusObservedGeneration(generation),
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceNoSufficientPermissions,
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDestURI,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDestURI,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
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
		Name: "valid with deprecated sink fields",
		Objects: []runtime.Object{
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &duckv1beta1.Destination{
						DeprecatedAPIVersion: sinkDest.Ref.APIVersion,
						DeprecatedKind:       sinkDest.Ref.Kind,
						DeprecatedName:       sinkDest.Ref.Name,
					},
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &duckv1beta1.Destination{
						DeprecatedAPIVersion: sinkDest.Ref.APIVersion,
						DeprecatedKind:       sinkDest.Ref.Kind,
						DeprecatedName:       sinkDest.Ref.Name,
					},
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSinkDepRef(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &duckv1beta1.Destination{
						Ref: sinkDest.Ref,
						URI: &apis.URL{Path: sinkURIReference},
					},
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeAvailableReceiveAdapterWithTargetURI(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &duckv1beta1.Destination{
						Ref: sinkDest.Ref,
						URI: &apis.URL{Path: sinkURIReference},
					},
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkTargetURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentEnv(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceDeploymentUnavailable,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapter(),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink:               &sinkDest,
					ServiceAccountName: "malin",
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentServiceAccount("morgan"),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink:               &sinkDest,
					ServiceAccountName: "malin",
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeploymentUnavailable,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapterWithDifferentServiceAccount("malin"),
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeReceiveAdapterWithDifferentContainerCount(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceDeploymentUpdated", "Deployment \"apiserversource-test-apiserver-source-1234\" updated"),
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{
						{
							APIVersion: "",
							Kind:       "Namespace",
						},
					},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeploymentUnavailable,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeReceiveAdapter(),
		}},
		WantCreates: []runtime.Object{
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with event types to delete",
		Objects: []runtime.Object{
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewChannel(sinkName, testNS,
				WithInitChannelConditions,
				WithChannelAddress(sinkDNS),
			),
			makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceAddEventType, "name-1"),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &sinkDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{Name: "name-1"},
		},
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
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewBroker(sinkName, testNS,
				WithInitBrokerConditions,
				WithBrokerAddress(sinkDNS),
			),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantCreates: []runtime.Object{
			makeEventType(sourcesv1alpha1.ApiServerSourceAddEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceDeleteEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceUpdateEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceAddRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceDeleteRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceUpdateRefEventType),
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with broker sink and missing event types",
		Objects: []runtime.Object{
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewBroker(sinkName, testNS,
				WithInitBrokerConditions,
				WithBrokerAddress(sinkDNS),
			),
			makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceAddEventType, "name-1"),
			makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceDeleteEventType, "name-2"),
			makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceUpdateEventType, "name-3"),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantCreates: []runtime.Object{
			makeEventType(sourcesv1alpha1.ApiServerSourceAddRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceDeleteRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceUpdateRefEventType),
			makeSubjectAccessReview("namespaces", "get", "default"),
			makeSubjectAccessReview("namespaces", "list", "default"),
			makeSubjectAccessReview("namespaces", "watch", "default"),
		},
		WithReactors:            []clientgotesting.ReactionFunc{subjectAccessReviewCreateReactor(true)},
		SkipNamespaceValidation: true, // SubjectAccessReview objects are cluster-scoped.
	}, {
		Name: "valid with broker sink and event types to delete",
		Objects: []runtime.Object{
			NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
			),
			NewBroker(sinkName, testNS,
				WithInitBrokerConditions,
				WithBrokerAddress(sinkDNS),
			),
			// https://github.com/knative/pkg/issues/411
			// Be careful adding more EventTypes here, the current unit test lister does not
			// return items in a fixed order, so the EventTypes can come back in any order.
			// WantDeletes requires the order to be correct, so will be flaky if we add more
			// than one EventType here.
			makeEventTypeWithName("type1", "name-1"),
			makeAvailableReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewApiServerSource(sourceName, testNS,
				WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
					Resources: []sourcesv1alpha1.ApiServerResource{{
						APIVersion: "",
						Kind:       "Namespace",
					}},
					Sink: &brokerDest,
				}),
				WithApiServerSourceUID(sourceUID),
				WithApiServerSourceObjectMetaGeneration(generation),
				// Status Update:
				WithInitApiServerSourceConditions,
				WithApiServerSourceDeployed,
				WithApiServerSourceSink(sinkURI),
				WithApiServerSourceSufficientPermissions,
				WithApiServerSourceEventTypes,
				WithApiServerSourceStatusObservedGeneration(generation),
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{Name: "name-1"},
		},
		WantCreates: []runtime.Object{
			makeEventType(sourcesv1alpha1.ApiServerSourceAddEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceDeleteEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceUpdateEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceAddRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceDeleteRefEventType),
			makeEventType(sourcesv1alpha1.ApiServerSourceUpdateRefEventType),
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
			Base:                  reconciler.NewBase(ctx, controllerAgentName, cmw),
			apiserversourceLister: listers.GetApiServerSourceLister(),
			eventTypeLister:       listers.GetEventTypeLister(),
			source:                source,
			receiveAdapterImage:   image,
			sinkResolver:          resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		}
		return apiserversource.NewReconciler(ctx, r.Logger, r.EventingClientSet, listers.GetApiServerSourceLister(), r.Recorder, r)
	},
		true,
		logger,
	))
}

func makeReceiveAdapter() *appsv1.Deployment {
	src := NewApiServerSource(sourceName, testNS,
		WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources: []sourcesv1alpha1.ApiServerResource{{
				APIVersion: "",
				Kind:       "Namespace",
			}},
			Sink: &sinkDest,
		}),
		WithApiServerSourceUID(sourceUID),
		// Status Update:
		WithInitApiServerSourceConditions,
		WithApiServerSourceDeployed,
		WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:   image,
		Source:  src,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI,
	}
	return resources.MakeReceiveAdapter(&args)
}

func makeAvailableReceiveAdapter() *appsv1.Deployment {
	ra := makeReceiveAdapter()
	WithDeploymentAvailable()(ra)
	return ra
}

func makeAvailableReceiveAdapterWithTargetURI() *appsv1.Deployment {
	src := NewApiServerSource(sourceName, testNS,
		WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources: []sourcesv1alpha1.ApiServerResource{
				{
					APIVersion: "",
					Kind:       "Namespace",
				},
			},
			Sink: &sinkDest,
		}),
		WithApiServerSourceUID(sourceUID),
		// Status Update:
		WithInitApiServerSourceConditions,
		WithApiServerSourceDeployed,
		WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:   image,
		Source:  src,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkTargetURI,
	}
	ra := resources.MakeReceiveAdapter(&args)
	WithDeploymentAvailable()(ra)
	return ra
}

func makeReceiveAdapterWithDifferentEnv() *appsv1.Deployment {
	ra := makeReceiveAdapter()
	ra.Spec.Template.Spec.Containers[0].Env = append(ra.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "not-in",
		Value: "the-original",
	})
	return ra
}

func makeReceiveAdapterWithDifferentServiceAccount(name string) *appsv1.Deployment {
	ra := makeReceiveAdapter()
	ra.Spec.Template.Spec.ServiceAccountName = name
	return ra
}

func makeReceiveAdapterWithDifferentContainerCount() *appsv1.Deployment {
	ra := makeReceiveAdapter()
	ra.Spec.Template.Spec.Containers = append(ra.Spec.Template.Spec.Containers, corev1.Container{})
	return ra
}

func makeEventTypeWithName(eventType, name string) *v1alpha1.EventType {
	et := makeEventType(eventType)
	et.Name = name
	return et
}

func makeEventType(eventType string) *v1alpha1.EventType {
	return &v1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", eventType, sourceUID),
			Labels:    resources.Labels(sourceName),
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(makeApiServerSource(), schema.GroupVersionKind{
					Group:   sourcesv1alpha1.SchemeGroupVersion.Group,
					Version: sourcesv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ApiServerSource",
				}),
			},
		},
		Spec: v1alpha1.EventTypeSpec{
			Type:   eventType,
			Source: source,
			Broker: sinkName,
		},
	}
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

func makeApiServerSource() *sourcesv1alpha1.ApiServerSource {
	return NewApiServerSource(sourceName, testNS,
		WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources: []sourcesv1alpha1.ApiServerResource{
				{
					APIVersion: "",
					Kind:       "Namespace",
				},
			},
			Sink: &brokerDest,
		}),
		WithApiServerSourceUID(sourceUID),
	)
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
