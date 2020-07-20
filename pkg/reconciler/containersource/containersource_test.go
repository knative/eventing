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

	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"

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

	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta1/containersource"
	"knative.dev/eventing/pkg/reconciler/containersource/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

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

	deploymentName  = fmt.Sprintf("%s-deployment", sourceName)
	sinkBindingName = fmt.Sprintf("%s-sinkbinding", sourceName)

	conditionTrue = corev1.ConditionTrue

	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1beta1",
		},
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
			Name: "error creating sink binding",
			Objects: []runtime.Object{
				NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "sinkbindings"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "creating new SinkBinding: inducing failure for %s %s", "create", "sinkbindings"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
					WithInitContainerSourceConditionsV1B1,
					WithContainerSourceStatusObservedGenerationV1B1(generation),
					WithContainerUnobservedGenerationV1B1(),
				),
			}},
			WantCreates: []runtime.Object{
				makeSinkBinding(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), nil),
			},
		}, {
			Name: "error creating deployment",
			Objects: []runtime.Object{
				NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, sinkBindingCreated, "SinkBinding created %q", sinkBindingName),
				Eventf(corev1.EventTypeWarning, "InternalError", "creating new Deployment: inducing failure for %s %s", "create", "deployments"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
					WithInitContainerSourceConditionsV1B1,
					WithContainerSourceStatusObservedGenerationV1B1(generation),
				),
			}},
			WantCreates: []runtime.Object{
				makeSinkBinding(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), nil),
				makeDeployment(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), nil),
			},
		}, {
			Name: "successfully reconciled and not ready",
			Objects: []runtime.Object{
				NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, sinkBindingCreated, "SinkBinding created %q", sinkBindingName),
				Eventf(corev1.EventTypeNormal, deploymentCreated, "Deployment created %q", deploymentName),
				Eventf(corev1.EventTypeNormal, sourceReconciled, "ContainerSource reconciled: \"%s/%s\"", testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
					WithInitContainerSourceConditionsV1B1,
					WithContainerSourceStatusObservedGenerationV1B1(generation),
					WithContainerSourcePropagateReceiveAdapterStatusV1B1(makeDeployment(NewContainerSourceV1Beta1(sourceName, testNS,
						WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
						WithContainerSourceUIDV1B1(sourceUID),
					), nil)),
				),
			}},
			WantCreates: []runtime.Object{
				makeSinkBinding(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), nil),
				makeDeployment(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), nil),
			},
		}, {
			Name: "successfully reconciled and ready",
			Objects: []runtime.Object{
				NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
				),
				makeSinkBinding(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), &conditionTrue),
				makeDeployment(NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceUIDV1B1(sourceUID),
				), &conditionTrue),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, sourceReconciled, "ContainerSource reconciled: \"%s/%s\"", testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSourceV1Beta1(sourceName, testNS,
					WithContainerSourceUIDV1B1(sourceUID),
					WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
					WithContainerSourceObjectMetaGenerationV1B1(generation),
					WithInitContainerSourceConditionsV1B1,
					WithContainerSourceStatusObservedGenerationV1B1(generation),
					WithContainerSourcePropagateSinkbindingStatusV1B1(makeSinkBindingStatus(&conditionTrue)),
					WithContainerSourcePropagateReceiveAdapterStatusV1B1(makeDeployment(NewContainerSourceV1Beta1(sourceName, testNS,
						WithContainerSourceSpecV1B1(makeContainerSourceSpec(sinkDest)),
						WithContainerSourceUIDV1B1(sourceUID),
					), &conditionTrue)),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:         fakekubeclient.Get(ctx),
			eventingClientSet:     fakeeventingclient.Get(ctx),
			containerSourceLister: listers.GetContainerSourceV1beta1Lister(),
			deploymentLister:      listers.GetDeploymentLister(),
			sinkBindingLister:     listers.GetSinkBindingV1beta1Lister(),
		}
		return containersource.NewReconciler(ctx, logging.FromContext(ctx), fakeeventingclient.Get(ctx), listers.GetContainerSourceV1beta1Lister(), controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeSinkBinding(source *sourcesv1beta1.ContainerSource, ready *corev1.ConditionStatus) *sourcesv1beta1.SinkBinding {
	sb := &sourcesv1beta1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      sinkBindingName,
			Namespace: source.Namespace,
		},
		Spec: sourcesv1beta1.SinkBindingSpec{
			SourceSpec: source.Spec.SourceSpec,
			BindingSpec: duckv1beta1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Namespace:  source.Namespace,
					Name:       resources.DeploymentName(source),
				},
			},
		},
	}
	if ready != nil {
		sb.Status = *makeSinkBindingStatus(ready)
	}
	return sb
}

func makeDeployment(source *sourcesv1beta1.ContainerSource, available *corev1.ConditionStatus) *appsv1.Deployment {
	template := source.Spec.Template

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
		APIVersion:         sourcesv1beta1.SchemeGroupVersion.String(),
		Kind:               "ContainerSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                sourceUID,
	}}
}

func makeContainerSourceSpec(sink duckv1.Destination) sourcesv1beta1.ContainerSourceSpec {
	return sourcesv1beta1.ContainerSourceSpec{
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

func makeSinkBindingStatus(ready *corev1.ConditionStatus) *sourcesv1beta1.SinkBindingStatus {
	return &sourcesv1beta1.SinkBindingStatus{
		SourceStatus: duckv1.SourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionReady,
					Status: *ready,
				}},
			},
		},
	}
}
