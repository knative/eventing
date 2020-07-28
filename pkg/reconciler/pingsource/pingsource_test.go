/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pingsource

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	recresources "knative.dev/eventing/pkg/reconciler/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	. "knative.dev/eventing/pkg/reconciler/testing"
	rtv1beta1 "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1beta1",
		},
	}
	sinkDestURI = duckv1.Destination{
		URI: apis.HTTP(sinkDNS),
	}
	sinkDNS    = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI, _ = apis.ParseURL("http://" + sinkDNS)
)

const (
	image          = "github.com/knative/test/image"
	mtimage        = "github.com/knative/test/mtimage"
	sourceName     = "test-ping-source"
	sourceUID      = "1234"
	sourceNameLong = "test-pingserver-source-with-a-very-long-name"
	sourceUIDLong  = "cafed00d-cafed00d-cafed00d-cafed00d-cafed00d"
	testNS         = "testnamespace"
	testSchedule   = "*/2 * * * *"
	testData       = "data"
	crName         = "knative-eventing-pingsource-adapter"

	sinkName   = "testsink"
	generation = 1
)

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "missing sink",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithPingSourceV1A2StatusObservedGeneration(generation),
					WithPingSourceV1A2SinkNotFound,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound",
					`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1beta1"}}`),
			},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceServiceAccountCreated", `PingSource ServiceAccount created`),
				Eventf(corev1.EventTypeNormal, "PingSourceRoleBindingCreated", `PingSource RoleBinding created`),
			},
			WantCreates: []runtime.Object{
				MakeServiceAccount(sourceName, sourceUID),
				MakeRoleBinding(sourceName, sourceUID),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with sink URI",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDestURI,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceServiceAccountCreated", `PingSource ServiceAccount created`),
				Eventf(corev1.EventTypeNormal, "PingSourceRoleBindingCreated", `PingSource RoleBinding created`),
			},
			WantCreates: []runtime.Object{
				MakeServiceAccount(sourceName, sourceUID),
				MakeRoleBinding(sourceName, sourceUID),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDestURI,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, existing ra",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceServiceAccountCreated", `PingSource ServiceAccount created`),
				Eventf(corev1.EventTypeNormal, "PingSourceRoleBindingCreated", `PingSource RoleBinding created`),
			},
			WantCreates: []runtime.Object{
				MakeServiceAccount(sourceName, sourceUID),
				MakeRoleBinding(sourceName, sourceUID),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, no change",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
				MakeServiceAccount(sourceName, sourceUID),
				MakeRoleBinding(sourceName, sourceUID),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceServiceAccountCreated", `PingSource ServiceAccount created`),
				Eventf(corev1.EventTypeNormal, "PingSourceRoleBindingCreated", `PingSource RoleBinding created`),
			},
			WantCreates: []runtime.Object{
				MakeServiceAccount(sourceName, sourceUID),
				MakeRoleBinding(sourceName, sourceUID),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with cluster scope annotation",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ClusterScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ClusterScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,

					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name:                    "valid with cluster scope annotation, create deployment",
			SkipNamespaceValidation: true,
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ClusterScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceDeploymentCreated", `Cluster-scoped deployment created`),
			},
			WantCreates: []runtime.Object{
				MakeMTAdapter(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ClusterScopeAnnotation,
					WithPingSourceV1A2UID(sourceUID),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithPingSourceV1A2NotDeployed(mtadapterName),
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
		}, {
			Name:                    "deprecated named adapter deployment found",
			SkipNamespaceValidation: true,
			Objects: []runtime.Object{
				NewPingSourceV1Alpha2(sourceNameLong, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUIDLong),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapterDeprecatedName(sourceNameLong, sourceUIDLong, sinkDest),
				MakeServiceAccount(sourceNameLong, sourceUIDLong),
				MakeRoleBinding(sourceNameLong, sourceUIDLong),
			},
			Key: testNS + "/" + sourceNameLong,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, pingSourceDeploymentDeleted, `Deprecated deployment removed: "%s/%s"`, testNS, makeAvailableReceiveAdapterDeprecatedName(sourceNameLong, sourceUIDLong, sinkDest).Name),
				Eventf(corev1.EventTypeNormal, "PingSourceDeploymentCreated", `Deployment created`),
			},
			WantCreates: []runtime.Object{
				// makeJobRunner(),
				makeReceiveAdapterWithSinkAndCustomData(sourceNameLong, sourceUIDLong, sinkDest),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha2(sourceNameLong, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					WithPingSourceV1A2ResourceScopeAnnotation,
					WithPingSourceV1A2UID(sourceUIDLong),
					WithPingSourceV1A2ObjectMetaGeneration(generation),
					// Status Update:
					WithPingSourceV1A2NotDeployed(makeReceiveAdapterWithSinkAndCustomData(sourceNameLong, sourceUIDLong, sinkDest).Name),
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2StatusObservedGeneration(generation),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: makeAvailableReceiveAdapterDeprecatedName(sourceNameLong, sourceUIDLong, sinkDest).Name,
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:         fakekubeclient.Get(ctx),
			pingLister:            listers.GetPingSourceV1alpha2Lister(),
			deploymentLister:      listers.GetDeploymentLister(),
			serviceAccountLister:  listers.GetServiceAccountLister(),
			roleBindingLister:     listers.GetRoleBindingLister(),
			tracker:               tracker.New(func(types.NamespacedName) {}, 0),
			receiveAdapterImage:   image,
			receiveMTAdapterImage: mtimage,
		}
		r.sinkResolver = resolver.NewURIResolver(ctx, func(types.NamespacedName) {})

		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceV1alpha2Lister(),
			controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeAvailableReceiveAdapter(dest duckv1.Destination) *appsv1.Deployment {
	ra := makeReceiveAdapterWithSink(dest)
	WithDeploymentAvailable()(ra)
	return ra
}

// makeAvailableReceiveAdapterDeprecatedName needed to simulate pre 0.14 adapter whose name was generated using utils.GenerateFixedName
func makeAvailableReceiveAdapterDeprecatedName(sourceName string, sourceUID string, dest duckv1.Destination) *appsv1.Deployment {
	ra := makeReceiveAdapterWithSink(dest)
	src := &sourcesv1alpha2.PingSource{}
	src.UID = types.UID(sourceUID)
	ra.Name = utils.GenerateFixedName(src, fmt.Sprintf("pingsource-%s", sourceName))
	WithDeploymentAvailable()(ra)
	return ra
}

func makeReceiveAdapterWithSinkAndCustomData(sourceName, sourceUID string, dest duckv1.Destination) *appsv1.Deployment {
	source := NewPingSourceV1Alpha2(sourceName, testNS,
		WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			Schedule: testSchedule,
			JsonData: testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: dest,
			},
		},
		),
		WithPingSourceV1A2UID(sourceUID),
		// Status Update:
		WithInitPingSourceV1A2Conditions,
		WithValidPingSourceV1A2Schedule,
		WithPingSourceV1A2Deployed,
		WithPingSourceV1A2Sink(sinkURI),
	)

	args := resources.Args{
		Image:   image,
		Source:  source,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI,
	}
	return resources.MakeReceiveAdapter(&args)
}

func makeReceiveAdapterWithSink(dest duckv1.Destination) *appsv1.Deployment {
	return makeReceiveAdapterWithSinkAndCustomData(sourceName, sourceUID, dest)
}

func MakeMTAdapter() *appsv1.Deployment {
	args := resources.MTArgs{
		ServiceAccountName: mtadapterName,
		MTAdapterName:      mtadapterName,
		Image:              mtimage,
	}
	return resources.MakeMTReceiveAdapter(args)
}

func makeAvailableMTAdapter() *appsv1.Deployment {
	ma := MakeMTAdapter()
	WithDeploymentAvailable()(ma)
	return ma
}

func MakeServiceAccount(sourceName, sourceUID string) *corev1.ServiceAccount {
	source := NewPingSourceV1Alpha2(sourceName, testNS,
		WithPingSourceV1A2UID(sourceUID))
	return recresources.MakeServiceAccount(source, resources.CreateReceiveAdapterName(sourceName, types.UID(sourceUID)))
}

func MakeRoleBinding(sourceName, sourceUID string) *rbacv1.RoleBinding {
	source := NewPingSourceV1Alpha2(sourceName, testNS,
		WithPingSourceV1A2UID(sourceUID))
	return resources.MakeRoleBinding(source, resources.CreateReceiveAdapterName(sourceName, types.UID(sourceUID)), crName)
}
