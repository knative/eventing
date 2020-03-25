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
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/pkg/apis"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/containersource"
	"knative.dev/eventing/pkg/reconciler/containersource/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/utils"

	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
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

	nonsinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Trigger",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}
	nonsinkDestWithNamespace = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Trigger",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}

	deploymentName = fmt.Sprintf("containersource-%s-%s", sourceName, sourceUID)

	// We cannot take the address of constants, so copy it into a var.
	conditionTrue = corev1.ConditionTrue

	serviceDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
	serviceURI = fmt.Sprintf("http://%s.%s.svc.cluster.local/", sinkName, testNS)

	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	sinkDestWithNamespace = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	sinkDNS     = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI     = apis.HTTP(sinkDNS)
	sinkDestURI = duckv1.Destination{
		URI: apis.HTTP(sinkDNS),
	}
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
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound", `Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1alpha1"}}`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(""),
				),
			}},
		}, {
			Name: "sink not addressable",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(nonsinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
				),
				NewTrigger(sinkName, testNS, ""),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound", `Sink not found: {"ref":{"kind":"Trigger","namespace":"testnamespace","name":"testsink","apiVersion":"eventing.knative.dev/v1alpha1"}}`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(nonsinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(""),
				),
			}},
		}, {
			Name: "sink not ready",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
				),
				NewChannel(sinkName, testNS),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound", `Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1alpha1"}}`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSinkNotFound(""),
				),
			}},
		}, {
			Name: "deployment unavailable",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGeneration(generation),
					WithContainerSourceUID(sourceUID),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourcePropagateDeploymentAvailability(
						makeDeployment(NewContainerSource(sourceName, testNS,
							WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
							WithContainerSourceUID(sourceUID),
						), nil, sinkURI, "", nil, nil)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
				), nil, sinkURI, "", nil, nil),
			},
		}, {
			Name: "valid with ready deployment",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
				), &conditionTrue, sinkURI, "", nil, nil),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					// Status Update:
					WithContainerSourcePropagateDeploymentAvailability(
						makeDeployment(NewContainerSource(sourceName, testNS,
							WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
							WithContainerSourceUID(sourceUID),
						), &conditionTrue, sinkURI, "", nil, nil)),
					WithContainerSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "error for create deployment",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
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
				Eventf(corev1.EventTypeWarning, "InternalError", "creating new deployment: inducing failure for create deployments"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
					WithContainerSourceObjectMetaGeneration(generation),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceStatusObservedGeneration(generation),
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeployFailed(`Failed to reconcile deployment: creating new deployment: inducing failure for create deployments`),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUID(sourceUID),
				), nil, sinkURI, "", nil, nil),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:         fakekubeclient.Get(ctx),
			eventingClientSet:     fakeeventingclient.Get(ctx),
			containerSourceLister: listers.GetContainerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
			sinkBindingLister:     listers.GetSinkBindingLister(),
		}
		return containersource.NewReconciler(ctx, logging.FromContext(ctx), fakeeventingclient.Get(ctx), listers.GetContainerSourceLister(), controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeDeployment(source *sourcesv1alpha1.ContainerSource, available *corev1.ConditionStatus, sinkURI *apis.URL, ceOverrides string, labels map[string]string, annotations map[string]string) *appsv1.Deployment {
	template := source.Spec.Template

	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Env = append(template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: sinkURI.String(),
		})
		template.Spec.Containers[i].Env = append(template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_CE_OVERRIDES",
			Value: ceOverrides,
		})
	}

	if template.Labels == nil {
		template.Labels = make(map[string]string)
	}
	for k, v := range resources.Labels(source.Name) {
		template.Labels[k] = v
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
			Labels:          resources.Labels(source.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: resources.Labels(source.Name),
			},
			Template: template,
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

func makeContainerSourceSpec(sink duckv1.Destination) sourcesv1alpha1.ContainerSourceSpec {
	return sourcesv1alpha1.ContainerSourceSpec{
		Template: corev1.PodTemplateSpec{
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
		SourceSpec: duckv1.SourceSpec{
			Sink: sink,
		},
	}
}
