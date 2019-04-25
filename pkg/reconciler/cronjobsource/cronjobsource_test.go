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
	"github.com/knative/eventing/pkg/reconciler/cronjobsource/resources"
	"github.com/knative/eventing/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"testing"

	clientgotesting "k8s.io/client-go/testing"

	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"

	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true

	sinkGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}
	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS + "/"
)

const (
	image        = "github.com/knative/test/image"
	sourceName   = "test-cronjob-source"
	sourceUID    = "1234-5678-90"
	testNS       = "testnamespace"
	testSchedule = "*/2 * * * *"
	testData     = "data"

	sinkName = "testsink"
)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
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
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
				),
			}},
			WantCreates: []metav1.Object{
				makeReceiveAdapter(),
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
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
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
					WithCronJobSourceDeployed,
					WithCronJobSourceSink(sinkURI),
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
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:             reconciler.NewBase(opt, controllerAgentName),
			cronjobLister:    listers.GetCronJobSourceLister(),
			deploymentLister: listers.GetDeploymentLister(),
		}
	}))

}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	cronjobInformer := eventingInformer.Sources().V1alpha1().CronJobSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		cronjobInformer,
		deploymentInformer,
	)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func makeReceiveAdapter() *v1.Deployment {
	source := NewCronSourceJob(sourceName, testNS,
		WithCronJobSourceSpec(sourcesv1alpha1.CronJobSourceSpec{
			Schedule: testSchedule,
			Data:     testData,
			Sink:     &sinkRef,
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
