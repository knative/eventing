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

package containersource

import (
	"fmt"
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
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/eventing/pkg/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-container-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	sinkName   = "testsink"
)

var (
	trueVal = true

	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	nonsinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Trigger",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS + "/"

	// TODO: k8s service does not work, fix.
	//serviceRef = corev1.ObjectReference{
	//	Name:       sinkName,
	//	Kind:       "Service",
	//	APIVersion: "v1",
	//}
	//serviceURI = "http://service.sink.svc.cluster.local/"
)

func init() {
	// Add types to scheme
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	containerSourceInformer := eventingInformer.Sources().V1alpha1().ContainerSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		containerSourceInformer,
		deploymentInformer,
	)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
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
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: Error fetching sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} for source "testnamespace/test-container-source, /, Kind=": channels.eventing.knative.dev "testsink" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": Error fetching sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} for source "testnamespace/test-container-source, /, Kind=": channels.eventing.knative.dev "testsink" not found"`),
				),
			}},
		}, {
			Name: "sink not addressable",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &nonsinkRef,
					}),
				),
				NewTrigger(sinkName, testNS, ""),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} does not contain address`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &nonsinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": sink &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} does not contain address"`),
				),
			}},
		}, {
			Name: "sink not ready",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} contains an empty hostname`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} contains an empty hostname"`),
				),
			}},
		}, {
			Name: "sink is nil",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: Sink missing from spec`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkMissing("Sink missing from spec"),
				),
			}},
		}, {
			Name: "valid first pass",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment ""`), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
			}},
			WantCreates: []metav1.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					WithContainerSourceUID(sourceUID),
				), 0, nil, nil),
			},
		}, {
			Name: "valid, with ready deployment",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					WithContainerSourceUID(sourceUID),
				), 1, nil, nil),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentReady", `Deployment "" has 1 ready replicas`),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReadinessChanged", `ContainerSource "test-container-source" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					// Status Update:
					WithContainerSourceDeployed,
				),
			}},
		}, {
			Name: "valid first pass, with annotations and labels",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment ""`), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
			}},
			WantCreates: []metav1.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					WithContainerSourceUID(sourceUID),
				), 0, map[string]string{"label": "labeled"}, map[string]string{"annotation": "annotated"}),
			},
		}, {
			Name: "error for create deployment",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "DeploymentCreateFailed", "Could not create deployment: inducing failure for create deployments"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeployFailed(`Could not create deployment: inducing failure for create deployments`),
				),
			}},
			WantCreates: []metav1.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					WithContainerSourceUID(sourceUID),
				), 0, nil, nil),
			},
		},
		//{ // TODO: k8s service does not work, fix.
		//	Name: "valid, with sink as service",
		//	Objects: []runtime.Object{
		//		NewContainerSource(sourceName, testNS,
		//			WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
		//				Image: image,
		//				Sink:  &serviceRef,
		//			}),
		//			WithContainerSourceUID(sourceUID),
		//		),
		//		NewService(sinkName, testNS),
		//	},
		//	Key: testNS + "/" + sourceName,
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewContainerSource(sourceName, testNS,
		//			WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
		//				Image: image,
		//				Sink:  &serviceRef,
		//			}),
		//			WithContainerSourceUID(sourceUID),
		//			// Status Update:
		//			WithInitContainerSourceConditions,
		//			WithContainerSourceSink(serviceURI),
		//			WithContainerSourceDeploying(`Created deployment ""`),
		//		),
		//	}},
		//	WantCreates: []metav1.Object{
		//		makeDeployment(NewContainerSource(sourceName, testNS,
		//			WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
		//				Image: image,
		//			}),
		//			WithContainerSourceUID(sourceUID),
		//		), 0),
		//	},
		//},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		r := &Reconciler{
			Base:                  reconciler.NewBase(opt, controllerAgentName),
			containerSourceLister: listers.GetContainerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
		}
		r.sinkReconciler = duck.NewSinkReconciler(opt, func(string){})
		return r
	},
	true,
	))
}

func makeDeployment(source *sourcesv1alpha1.ContainerSource, replicas int32, labels map[string]string, annotations map[string]string) *appsv1.Deployment {
	args := append(source.Spec.Args, fmt.Sprintf("--sink=%s", sinkURI))
	env := append(source.Spec.Env, corev1.EnvVar{Name: "SINK", Value: sinkURI})

	labs := map[string]string{
		"eventing.knative.dev/source": source.Name,
	}
	for k, v := range labels {
		labs[k] = v
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    fmt.Sprintf("%s-", source.Name),
			Namespace:       source.Namespace,
			OwnerReferences: getOwnerReferences(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/source": source.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labs,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "source",
						Image:           source.Spec.Image,
						Args:            args,
						Env:             env,
						ImagePullPolicy: corev1.PullIfNotPresent,
					}},
					ServiceAccountName: source.Spec.ServiceAccountName,
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: replicas,
		},
	}
}

func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:               "ContainerSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                sourceUID,
	}}
}
