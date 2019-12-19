/*
Copyright 2019 The Knative Authors

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

package legacycronjobsource

import (
	"context"
	"fmt"
	"testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/resolver"

	"knative.dev/pkg/configmap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	"knative.dev/pkg/controller"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/legacycronjobsource/resources"
	"knative.dev/eventing/pkg/utils"

	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
)

var (
	trueVal  = true
	ownerRef = metav1.OwnerReference{
		APIVersion:         "sources.eventing.knative.dev/v1alpha1",
		Kind:               "CronJobSource",
		Name:               sourceName,
		UID:                sourceUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
	eventTypeName = fmt.Sprintf("dev.knative.cronjob.event-%s", sourceUID)

	sinkDest = duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	sinkDestURI = duckv1beta1.Destination{
		URI: apis.HTTP(sinkDNS),
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
)

const (
	image        = "github.com/knative/test/image"
	sourceName   = "test-cronjob-source"
	sourceUID    = "1234"
	testNS       = "testnamespace"
	testSchedule = "*/2 * * * *"
	testData     = "data"

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
			Name: "invalid schedule",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: "invalid schedule",
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "CronJobSourceUpdateStatusFailed",
					"Failed to update CronJobSource's status: invalid schedule: expected exactly 5 fields, found 2: [invalid schedule]"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: "invalid schedule",
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithCronJobSourceStatusObservedGeneration(generation),
					WithInvalidCronJobSourceSchedule,
					WithCronJobSourceSink(sinkURI),
				),
			}},
		}, {
			Name: "missing sink",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithCronJobSourceStatusObservedGeneration(generation),
					WithCronJobSourceSinkNotFound,
				),
			}},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with sink URI",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDestURI,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDestURI,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with event type creation",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(brokerDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceEventType,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
			},
		}, {
			Name: "valid with event type deletion and creation",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
				NewEventType(eventTypeName, testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType("type-1"),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
				makeAvailableReceiveAdapter(brokerDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceEventType,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: eventTypeName,
			}},
			WantCreates: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
			},
		}, {
			Name: "valid, existing ra",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, no change",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
			},
		}, {
			Name: "valid with event type deletion",
			Objects: []runtime.Object{
				NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeAvailableReceiveAdapter(sinkDest),
				NewEventType(eventTypeName, testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronJobSource(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkDest,
					}),
					WithCronJobSourceUID(sourceUID),
					WithCronJobSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
					WithCronJobSourceStatusObservedGeneration(generation),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: eventTypeName,
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			cronjobLister:       listers.GetLegacyCronJobSourceLister(),
			deploymentLister:    listers.GetDeploymentLister(),
			eventTypeLister:     listers.GetEventTypeLister(),
			receiveAdapterImage: image,
		}
		r.sinkResolver = resolver.NewURIResolver(ctx, func(types.NamespacedName) {})

		return r
	},
		true,
		logger,
	))
}

func makeAvailableReceiveAdapter(dest duckv1beta1.Destination) *appsv1.Deployment {
	ra := makeReceiveAdapterWithSink(dest)
	WithDeploymentAvailable()(ra)
	return ra
}

func makeReceiveAdapterWithSink(dest duckv1beta1.Destination) *appsv1.Deployment {
	source := NewCronJobSource(sourceName, testNS,
		WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
			Schedule: testSchedule,
			Data:     testData,
			Sink:     &dest,
		},
		),
		WithCronJobSourceUID(sourceUID),
		// Status Update:
		WithInitCronJobSourceConditions,
		WithValidCronJobSourceSchedule,
		WithCronJobSourceDeployed,
		WithCronJobSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:   image,
		Source:  source,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI,
	}
	return resources.MakeReceiveAdapter(&args)
}
