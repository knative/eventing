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
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/utils"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-container-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	sinkName   = "testsink"
	generation = 1
)

var (
	trueVal = true

	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "messaging.knative.dev/v1alpha1",
	}
	nonsinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Trigger",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS

	deploymentName = fmt.Sprintf("containersource-%s-%s", sourceName, sourceUID)

	// We cannot take the address of constants, so copy it into a var.
	conditionTrue = corev1.ConditionTrue

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
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: Error fetching sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} for source "testnamespace/test-container-source, /, Kind=": channels.messaging.knative.dev "testsink" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": Error fetching sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} for source "testnamespace/test-container-source, /, Kind=": channels.messaging.knative.dev "testsink" not found"`),
				),
			}},
		}, {
			Name: "sink not addressable",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &nonsinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
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
						DeprecatedImage: image,
						Sink:            &nonsinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": sink &ObjectReference{Kind:Trigger,Namespace:testnamespace,Name:testsink,UID:,APIVersion:eventing.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} does not contain address"`),
				),
			}},
		}, {
			Name: "sink not ready",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} does not contain address`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "testnamespace/testsink": sink &ObjectReference{Kind:Channel,Namespace:testnamespace,Name:testsink,UID:,APIVersion:messaging.knative.dev/v1alpha1,ResourceVersion:,FieldPath:,} does not contain address"`),
				),
			}},
		}, {
			Name: "sink is nil",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink missing from spec`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkMissing("Sink missing from spec"),
				),
			}},
		}, {
			Name: "valid first pass with template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceUID(sourceUID),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment "%s"`, deploymentName), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(fmt.Sprintf(`Created deployment "%s"`, deploymentName)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), nil, nil, nil),
			},
		}, {
			Name: "valid first pass without template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment "%s"`, deploymentName), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceUID(sourceUID),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(fmt.Sprintf(`Created deployment "%s"`, deploymentName)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), nil, nil, nil),
			},
		}, {
			Name: "valid, with ready deployment with template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), &conditionTrue, nil, nil),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentReady", `Deployment "%s" has 1 ready replicas`, deploymentName),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReadinessChanged", `ContainerSource "test-container-source" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					// Status Update:
					WithContainerSourceDeployed,
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid, with ready deployment without template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), &conditionTrue, nil, nil),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentReady", `Deployment "%s" has 1 ready replicas`, deploymentName),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
				Eventf(corev1.EventTypeNormal, "ContainerSourceReadinessChanged", `ContainerSource "test-container-source" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					// Status Update:
					WithContainerSourceDeployed,
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid first pass, with annotations and labels with template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment "%s"`, deploymentName), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Template: &corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "source",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
								},
							},
						},
						Sink: &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(fmt.Sprintf(`Created deployment "%s"`, deploymentName)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), nil, map[string]string{"label": "labeled"}, map[string]string{"annotation": "annotated"}),
			},
		}, {
			Name: "valid first pass, with annotations and labels without template",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment "%s"`, deploymentName), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceLabels(map[string]string{"label": "labeled"}),
					WithContainerSourceAnnotations(map[string]string{"annotation": "annotated"}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(fmt.Sprintf(`Created deployment "%s"`, deploymentName)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), nil, map[string]string{"label": "labeled"}, map[string]string{"annotation": "annotated"}),
			},
		}, {
			Name: "error for create deployment",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
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
						DeprecatedImage: image,
						Sink:            &sinkRef,
					}),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeployFailed(`Could not create deployment: inducing failure for create deployments`),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						DeprecatedImage: image,
					}),
					WithContainerSourceUID(sourceUID),
				), nil, nil, nil),
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
		//	WantCreates: []runtime.Object{
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
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			Base:                  reconciler.NewBase(ctx, controllerAgentName, cmw),
			containerSourceLister: listers.GetContainerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
		}
		r.sinkReconciler = duck.NewSinkReconciler(ctx, func(string) {})
		return r
	},
		true,
	))
}

func makeDeployment(source *sourcesv1alpha1.ContainerSource, available *corev1.ConditionStatus, labels map[string]string, annotations map[string]string) *appsv1.Deployment {
	args := append(source.Spec.DeprecatedArgs, fmt.Sprintf("--sink=%s", sinkURI))
	env := append(source.Spec.DeprecatedEnv, corev1.EnvVar{Name: "SINK", Value: sinkURI})

	labs := map[string]string{
		"sources.eventing.knative.dev/containerSource": source.Name,
	}
	for k, v := range labels {
		labs[k] = v
	}

	status := appsv1.DeploymentStatus{}
	if available != nil {
		status.Conditions = []appsv1.DeploymentCondition{
			{
				Type:   appsv1.DeploymentAvailable,
				Status: *available,
			},
		}
		if *available == corev1.ConditionTrue {
			status.ReadyReplicas = 1
		}
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Namespace:       source.Namespace,
			OwnerReferences: getOwnerReferences(),
			Labels: map[string]string{
				"sources.eventing.knative.dev/containerSource": source.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"sources.eventing.knative.dev/containerSource": source.Name,
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
						Image:           source.Spec.DeprecatedImage,
						Args:            args,
						Env:             env,
						ImagePullPolicy: corev1.PullIfNotPresent,
					}},
					ServiceAccountName: source.Spec.DeprecatedServiceAccountName,
				},
			},
		},
		Status: status,
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
