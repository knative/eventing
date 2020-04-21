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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/pingsource"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	. "knative.dev/eventing/pkg/reconciler/testing"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	sinkDestURI = duckv1.Destination{
		URI: apis.HTTP(sinkDNS),
	}
	sinkDNS    = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI, _ = apis.ParseURL("http://" + sinkDNS)
)

const (
	mtimage        = "github.com/knative/test/mtimage"
	sourceName     = "test-ping-source"
	sourceUID      = "1234"
	sourceNameLong = "test-pingserver-source-with-a-very-long-name"
	sourceUIDLong  = "cafed00d-cafed00d-cafed00d-cafed00d-cafed00d"
	testNS         = "testnamespace"
	testSchedule   = "*/2 * * * *"
	testData       = "data"

	sinkName   = "testsink"
	generation = 1
)

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
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
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithPingSourceStatusObservedGeneration(generation),
					WithPingSourceSinkNotFound,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound",
					`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1alpha1"}}`),
			},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with sink URI",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDestURI,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDestURI,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, existing adapter",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, no change",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
		}, {
			Name: "valid with cluster scope annotation",
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceDeployed,
					WithPingSourceSink(sinkURI),
					WithPingSourceEventType,
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name:                    "valid create MTAdapter",
			SkipNamespaceValidation: true,
			Objects: []runtime.Object{
				NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceDeploymentCreated", `Cluster-scoped deployment created`),
				Eventf(corev1.EventTypeNormal, "PingSourceReconciled", `PingSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantCreates: []runtime.Object{
				MakeAdapter(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPingSourceV1Alpha1(sourceName, testNS,
					WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithPingSourceUID(sourceUID),
					WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					WithPingSourceNotDeployed(mtadapterName),
					WithInitPingSourceConditions,
					WithValidPingSourceSchedule,
					WithValidPingSourceResources,
					WithPingSourceEventType,
					WithPingSourceSink(sinkURI),
					WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:       fakekubeclient.Get(ctx),
			pingLister:          listers.GetPingSourceLister(),
			deploymentLister:    listers.GetDeploymentLister(),
			tracker:             tracker.New(func(types.NamespacedName) {}, 0),
			receiveAdapterImage: mtimage,
		}
		r.sinkResolver = resolver.NewURIResolver(ctx, func(types.NamespacedName) {})

		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceLister(),
			controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func MakeAdapter() *appsv1.Deployment {
	args := resources.Args{
		ServiceAccountName: mtadapterName,
		AdapterName:        mtadapterName,
		Image:              mtimage,
	}
	return resources.MakeReceiveAdapter(args)
}

func makeAvailableAdapter() *appsv1.Deployment {
	ma := MakeAdapter()
	WithDeploymentAvailable()(ma)
	return ma
}
