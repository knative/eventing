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

package cronjobsource

import (
	"context"
	"fmt"
	"os"
	"testing"

	"knative.dev/pkg/configmap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/cronjobsource/resources"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	brokerRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS

	trueVal  = true
	ownerRef = metav1.OwnerReference{
		APIVersion:         "sources.eventing.knative.dev/v1alpha1",
		Kind:               "CronJobSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
)

const (
	image        = "github.com/knative/test/image"
	sourceName   = "test-cronjob-source"
	testNS       = "testnamespace"
	testSchedule = "*/2 * * * *"
	testData     = "data"

	sinkName = "testsink"
)

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)

	_ = os.Setenv("CRONJOB_RA_IMAGE", image)
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
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: "invalid schedule",
						Data:     testData,
						Sink:     &sinkRef,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			//WantEvents: []string{
			//	Eventf(corev1.EventTypeWarning, "Fail", ""), // TODO: BUGBUGBUG This should make an event.
			//},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: "invalid schedule",
						Data:     testData,
						Sink:     &sinkRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithInvalidCronJobSourceSchedule,
				),
			}},
		}, {
			Name: "missing sink",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithCronJobSourceSinkNotFound,
				),
			}},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
				),
			}},
			WantCreates: []runtime.Object{
				makeReceiveAdapter(),
			},
		}, {
			Name: "valid with event type creation",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerRef,
					}),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceEventType,
					WithCronJobSourceSink(sinkURI),
				),
			}},
			WantCreates: []runtime.Object{
				NewEventType("", testNS,
					WithEventTypeGenerateName(fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(sourcesv1alpha1.CronJobEventType))),
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
				makeReceiveAdapterWithSink(brokerRef),
			},
		}, {
			Name: "valid with event type deletion and creation",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerRef,
					}),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
				NewEventType("name-1", testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType("type-1"),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &brokerRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceEventType,
					WithCronJobSourceSink(sinkURI),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "name-1",
			}},
			WantCreates: []runtime.Object{
				NewEventType("", testNS,
					WithEventTypeGenerateName(fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(sourcesv1alpha1.CronJobEventType))),
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
				makeReceiveAdapterWithSink(brokerRef),
			},
		}, {
			Name: "valid, existing ra",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeReceiveAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "CronJobSourceReadinessChanged", `CronJobSource %q became ready`, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
					// Status Update:
					WithInitCronJobSourceConditions,
					WithValidCronJobSourceSchedule,
					WithValidCronJobSourceResources,
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
					WithCronJobSourceEventType,
				),
			}},
		}, {
			Name: "valid, no change",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
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
				makeReceiveAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
			},
		}, {
			Name: "valid with event type deletion",
			Objects: []runtime.Object{
				NewCronSourceJob(sourceName, testNS,
					WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
						Schedule: testSchedule,
						Data:     testData,
						Sink:     &sinkRef,
					}),
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
				makeReceiveAdapter(),
				NewEventType("name-1", testNS,
					WithEventTypeLabels(resources.Labels(sourceName)),
					WithEventTypeType(sourcesv1alpha1.CronJobEventType),
					WithEventTypeSource(sourcesv1alpha1.CronJobEventSource(testNS, sourceName)),
					WithEventTypeBroker(sinkName),
					WithEventTypeOwnerReference(ownerRef)),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "CronJobSourceReconciled", `CronJobSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "name-1",
			}},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
			cronjobLister:    listers.GetCronJobSourceLister(),
			deploymentLister: listers.GetDeploymentLister(),
			eventTypeLister:  listers.GetEventTypeLister(),
		}
		r.sinkReconciler = duck.NewSinkReconciler(ctx, func(string) {})
		return r
	},
		true,
	))
}

func makeReceiveAdapter() *appsv1.Deployment {
	return makeReceiveAdapterWithSink(sinkRef)
}

func makeReceiveAdapterWithSink(ref corev1.ObjectReference) *appsv1.Deployment {
	source := NewCronSourceJob(sourceName, testNS,
		WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
			Schedule: testSchedule,
			Data:     testData,
			Sink:     &ref,
		},
		),
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
