/*
Copyright 2019 The Knative Authors

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
	"fmt"
	"os"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/apiserversource/resources"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"
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
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-apiserver-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"

	sinkName = "testsink"
	source   = "apiserveraddr"
)

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)

	_ = os.Setenv("APISERVER_RA_IMAGE", image)
}

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "missing sink",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Sink: &sinkRef,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Sink: &sinkRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceSinkNotFound,
				),
			}},
		},
		{
			Name: "valid",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Resources: []sourcesv1alpha1.ApiServerResource{
							{
								APIVersion: "",
								Kind:       "Namespace",
							},
						},
						Sink: &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReadinessChanged", `ApiServerSource %q became ready`, sourceName),
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
						Sink: &sinkRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceDeployed,
					WithApiServerSourceSink(sinkURI),
					WithApiServerSourceEventTypes,
				),
			}},
			WantCreates: []runtime.Object{
				makeReceiveAdapter(),
			},
		},
		{
			Name: "valid with event types to delete",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Resources: []sourcesv1alpha1.ApiServerResource{
							{
								APIVersion: "",
								Kind:       "Namespace",
							},
						},
						Sink: &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS,
					WithInitChannelConditions,
					WithChannelAddress(sinkDNS),
				),
				makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceAddEventType, "name-1"),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReadinessChanged", `ApiServerSource %q became ready`, sourceName),
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
						Sink: &sinkRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceDeployed,
					WithApiServerSourceSink(sinkURI),
					WithApiServerSourceEventTypes,
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{Name: "name-1"},
			},
			WantCreates: []runtime.Object{
				makeReceiveAdapter(),
			},
		},
		{
			Name: "valid with broker sink",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Resources: []sourcesv1alpha1.ApiServerResource{
							{
								APIVersion: "",
								Kind:       "Namespace",
							},
						},
						Sink: &brokerRef,
					}),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReadinessChanged", `ApiServerSource %q became ready`, sourceName),
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
						Sink: &brokerRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceDeployed,
					WithApiServerSourceSink(sinkURI),
					WithApiServerSourceEventTypes,
				),
			}},
			WantCreates: []runtime.Object{
				makeEventType(sourcesv1alpha1.ApiServerSourceAddEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceDeleteEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceUpdateEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceAddRefEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceDeleteRefEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceUpdateRefEventType),
				makeReceiveAdapter(),
			},
		},
		{
			Name: "valid with broker sink and missing event types",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Resources: []sourcesv1alpha1.ApiServerResource{
							{
								APIVersion: "",
								Kind:       "Namespace",
							},
						},
						Sink: &brokerRef,
					}),
				),
				NewBroker(sinkName, testNS,
					WithInitBrokerConditions,
					WithBrokerAddress(sinkDNS),
				),
				makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceAddEventType, "name-1"),
				makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceDeleteEventType, "name-2"),
				makeEventTypeWithName(sourcesv1alpha1.ApiServerSourceUpdateEventType, "name-3"),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReadinessChanged", `ApiServerSource %q became ready`, sourceName),
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
						Sink: &brokerRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceDeployed,
					WithApiServerSourceSink(sinkURI),
					WithApiServerSourceEventTypes,
				),
			}},
			WantCreates: []runtime.Object{
				makeEventType(sourcesv1alpha1.ApiServerSourceAddRefEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceDeleteRefEventType),
				makeEventType(sourcesv1alpha1.ApiServerSourceUpdateRefEventType),
				makeReceiveAdapter(),
			},
		},
		{
			Name: "valid with broker sink and event types to delete",
			Objects: []runtime.Object{
				NewApiServerSource(sourceName, testNS,
					WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
						Resources: []sourcesv1alpha1.ApiServerResource{
							{
								APIVersion: "",
								Kind:       "Namespace",
							},
						},
						Sink: &brokerRef,
					}),
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
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReconciled", `ApiServerSource reconciled: "%s/%s"`, testNS, sourceName),
				Eventf(corev1.EventTypeNormal, "ApiServerSourceReadinessChanged", `ApiServerSource %q became ready`, sourceName),
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
						Sink: &brokerRef,
					}),
					// Status Update:
					WithInitApiServerSourceConditions,
					WithApiServerSourceDeployed,
					WithApiServerSourceSink(sinkURI),
					WithApiServerSourceEventTypes,
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
				makeReceiveAdapter(),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		r := &Reconciler{
			Base:                  reconciler.NewBase(opt, controllerAgentName),
			apiserversourceLister: listers.GetApiServerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
			source:                source,
		}
		r.sinkReconciler = duck.NewSinkReconciler(opt, func(string) {})
		return r
	},
		true,
	))
}
func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	apiserverInformer := eventingInformer.Sources().V1alpha1().ApiServerSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()
	eventTypeInformer := eventingInformer.Eventing().V1alpha1().EventTypes()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		apiserverInformer,
		deploymentInformer,
		eventTypeInformer,
		source,
	)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func makeReceiveAdapter() *appsv1.Deployment {
	source := NewApiServerSource(sourceName, testNS,
		WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources: []sourcesv1alpha1.ApiServerResource{
				{
					APIVersion: "",
					Kind:       "Namespace",
				},
			},
			Sink: &sinkRef,
		},
		),
		// Status Update:
		WithInitApiServerSourceConditions,
		WithApiServerSourceDeployed,
		WithApiServerSourceSink(sinkURI),
	)

	args := resources.ReceiveAdapterArgs{
		Image:   image,
		Source:  source,
		Labels:  resources.Labels(sourceName),
		SinkURI: sinkURI,
	}
	return resources.MakeReceiveAdapter(&args)
}

func makeEventTypeWithName(eventType, name string) *v1alpha1.EventType {
	et := makeEventType(eventType)
	et.Name = name
	return et
}

func makeEventType(eventType string) *v1alpha1.EventType {
	return &v1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(eventType)),
			Labels:       resources.Labels(sourceName),
			Namespace:    testNS,
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

func makeApiServerSource() *sourcesv1alpha1.ApiServerSource {
	return NewApiServerSource(sourceName, testNS,
		WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources: []sourcesv1alpha1.ApiServerResource{
				{
					APIVersion: "",
					Kind:       "Namespace",
				},
			},
			Sink: &brokerRef,
		}),
	)
}
