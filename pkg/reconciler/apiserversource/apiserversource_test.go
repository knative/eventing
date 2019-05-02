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
	"os"
	"testing"

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
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS + "/"
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-apiserver-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"

	sinkName = "testsink"
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
				),
			}},
			WantCreates: []metav1.Object{
				makeReceiveAdapter(),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                  reconciler.NewBase(opt, controllerAgentName),
			apiserversourceLister: listers.GetApiServerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
		}
	}))
}
func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	apiserverInformer := eventingInformer.Sources().V1alpha1().ApiServerSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		apiserverInformer,
		deploymentInformer,
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
