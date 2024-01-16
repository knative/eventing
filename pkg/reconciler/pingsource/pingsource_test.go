/*
Copyright 2021 The Knative Authors

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

package pingsource

import (
	"context"
	"fmt"
	"os"
	"testing"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/adapter/v2"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
	rtv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}
	sinkDestNoNS = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  "",
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}
	sinkDNS         = "sink.mynamespace.svc." + network.GetClusterDomainName()
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
	sourceName      = "test-ping-source"
	sourceUID       = "1234"
	testNS          = "testnamespace"
	testSchedule    = "*/2 * * * *"
	testContentType = cloudevents.TextPlain
	testData        = "data"
	testDataBase64  = "ZGF0YQ==" // "data"

	sinkName   = "testsink"
	generation = 1
)

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
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceSinkNotFound,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeWarning, "SinkNotFound",
					`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1"}}`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		}, {
			Name: "sink ref has no namespace",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDestNoNS,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDestNoNS,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceSinkNotFound,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeWarning, "SinkNotFound",
					`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1"}}`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		}, {
			Name: "error creating deployment",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeWarning, "InternalError", "deployments.apps \"pingsource-mt-adapter\" not found"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceSink(sinkAddressable),
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "no deployment update due to additional env variable",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableMTAdapter(WithContainerEnv("KUBERNETES_MIN_VERSION", "v1.0.0")),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddressable),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		}, {
			Name: "Propagate CA certs",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(&duckv1.Addressable{
						Name:    sinkAddressable.Name,
						URL:     sinkAddressable.URL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
				),
				makeAvailableMTAdapter(WithContainerEnv("KUBERNETES_MIN_VERSION", "v1.0.0")),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(&duckv1.Addressable{
						Name:    sinkAddressable.Name,
						URL:     sinkAddressable.URL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		}, {
			Name: "deployment update due to env",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableMTAdapterWithDifferentEnv(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeNormal, pingSourceDeploymentUpdated, `PingSource adapter deployment updated`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddressable),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: makeAvailableMTAdapter(),
			}},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddressable),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		}, {
			Name: "valid with dataBase64",
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddressable),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		},
		{
			Name: "OIDC: creates OIDC service account",
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkOIDCDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkOIDCAddressable),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkOIDCDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkOIDCAddressable),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedSucceeded(),
					rtv1.WithPingSourceOIDCServiceAccountName(makePingSourceOIDCServiceAccount().Name),
				),
			}},
			WantCreates: []runtime.Object{
				makePingSourceOIDCServiceAccount(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		},
		{
			Name: "OIDC: PingSource not ready on invalid OIDC service account",
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				makePingSourceOIDCServiceAccountWithoutOwnerRef(),
				rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1.NewChannel(sinkName, testNS,
					rtv1.WithInitChannelConditions,
					rtv1.WithChannelAddress(sinkAddressable),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1.NewPingSource(sourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1.WithPingSource(sourceUID),
					rtv1.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceStatusObservedGeneration(generation),
					rtv1.WithPingSourceOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", fmt.Sprintf("service account %s not owned by PingSource %s", makePingSourceOIDCServiceAccountWithoutOwnerRef().Name, sourceName)),
					rtv1.WithPingSourceOIDCServiceAccountName(makePingSourceOIDCServiceAccount().Name),
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("service account %s not owned by PingSource %s", makePingSourceOIDCServiceAccountWithoutOwnerRef().Name, sourceName)),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(sourceName, testNS),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, rtv1.MakeFactory(func(ctx context.Context, listers *rtv1.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			configAcc:            &reconcilersource.EmptyVarsGenerator{},
			kubeClientSet:        fakekubeclient.Get(ctx),
			tracker:              tracker.New(func(types.NamespacedName) {}, 0),
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		r.sinkResolver = resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0))

		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceLister(),
			controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func MakeMTAdapter() *appsv1.Deployment {
	args := resources.Args{
		ConfigEnvVars:   (&reconcilersource.EmptyVarsGenerator{}).ToEnvVars(),
		NoShutdownAfter: mtping.GetNoShutDownAfterValue(),
		SinkTimeout:     adapter.GetSinkTimeout(nil),
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      mtadapterName,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: containerName,
							Env:  resources.MakeReceiveAdapterEnvVar(args),
						}}}}}}

}

func makeAvailableMTAdapter(options ...DeploymentOption) *appsv1.Deployment {
	ma := MakeMTAdapter()
	for _, opt := range append(options, WithDeploymentAvailable()) {
		opt(ma)
	}
	return ma
}

func makeAvailableMTAdapterWithDifferentEnv() *appsv1.Deployment {
	os.Setenv("K_SINK_TIMEOUT", "500")
	ma := MakeMTAdapter()
	os.Unsetenv("K_SINK_TIMEOUT")
	WithDeploymentAvailable()(ma)
	return ma
}

func patchFinalizers(name, namespace string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["pingsources.sources.knative.dev"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func makePingSourceOIDCServiceAccount() *corev1.ServiceAccount {
	return auth.GetOIDCServiceAccountForResource(sourcesv1.SchemeGroupVersion.WithKind("PingSource"), metav1.ObjectMeta{
		Name:      sourceName,
		Namespace: testNS,
		UID:       sourceUID,
	})
}

func makePingSourceOIDCServiceAccountWithoutOwnerRef() *corev1.ServiceAccount {
	sa := auth.GetOIDCServiceAccountForResource(sourcesv1.SchemeGroupVersion.WithKind("PingSource"), metav1.ObjectMeta{
		Name:      sourceName,
		Namespace: testNS,
		UID:       sourceUID,
	})
	sa.OwnerReferences = nil

	return sa
}
