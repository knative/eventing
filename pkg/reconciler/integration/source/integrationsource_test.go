/*
Copyright 2024 The Knative Authors

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

package source

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/integrationsource"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"context"

	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	"testing"
)

const (
	sourceName = "test-integration-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	sinkName   = "testsink"
	generation = 1
)

var (
	conditionTrue = corev1.ConditionTrue

	containerSourceName = fmt.Sprintf("%s-containersource", sourceName)

	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}
)

func TestReconcile(t *testing.T) {

	table := TableTest{
		{
			Name: "bad work queue key",
			Key:  "too/many/parts",
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "error creating containersource",
			Objects: []runtime.Object{
				NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
				),
			},
			Key: testNS + "/" + sourceName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "containersources"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "creating new ContainerSource: inducing failure for %s %s", "create", "containersources"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
					WithInitIntegrationSourceConditions,
				),
			}},
			WantCreates: []runtime.Object{
				makeContainerSource(NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest))),
					nil),
			},
		}, {
			Name: "successfully reconciled and not ready",
			Objects: []runtime.Object{
				NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, containerSourceCreated, "ContainerSource created %q", containerSourceName),
				Eventf(corev1.EventTypeNormal, sourceReconciled, `IntegrationSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
					WithInitIntegrationSourceConditions,
				),
			}},
			WantCreates: []runtime.Object{
				makeContainerSource(NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest))),
					nil),
			},
		}, {
			Name: "successfully reconciled and ready",
			Objects: []runtime.Object{
				NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
				),
				makeContainerSource(NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
				), &conditionTrue),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, sourceReconciled, `IntegrationSource reconciled: "%s/%s"`, testNS, sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSource(sourceName, testNS,
					WithIntegrationSourceUID(sourceUID),
					WithIntegrationSourceSpec(makeIntegrationSourceSpec(sinkDest)),
					WithInitIntegrationSourceConditions,
					WithIntegrationSourceStatusObservedGeneration(generation),
					WithIntegrationSourcePropagateContainerSourceStatus(makeContainerSourceStatus(&conditionTrue)),
				),
			}},
		}}
	logger := logtesting.TestLogger(t)

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{

			kubeClientSet:           fakekubeclient.Get(ctx),
			eventingClientSet:       fakeeventingclient.Get(ctx),
			containerSourceLister:   listers.GetContainerSourceLister(),
			integrationSourceLister: listers.GetIntegrationSourceLister(),
		}

		return integrationsource.NewReconciler(ctx, logging.FromContext(ctx), fakeeventingclient.Get(ctx), listers.GetIntegrationSourceLister(), controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeContainerSource(source *sourcesv1alpha1.IntegrationSource, ready *corev1.ConditionStatus) runtime.Object {
	cs := &sourcesv1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      containerSourceName,
			Namespace: source.Namespace,
		},
		Spec: sourcesv1.ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           "gcr.io/knative-nightly/timer-source:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "CAMEL_KNATIVE_CLIENT_SSL_ENABLED",
									Value: "true",
								},
								{
									Name:  "CAMEL_KNATIVE_CLIENT_SSL_CERT_PATH",
									Value: "/knative-custom-certs/knative-eventing-bundle.pem",
								},
								{
									Name:  "CAMEL_KAMELET_TIMER_SOURCE_PERIOD",
									Value: "1000",
								},
								{
									Name:  "CAMEL_KAMELET_TIMER_SOURCE_MESSAGE",
									Value: "Hallo",
								},
								{
									Name:  "CAMEL_KAMELET_TIMER_SOURCE_REPEATCOUNT",
									Value: "0",
								},
							},
						},
					},
				},
			},
			SourceSpec: source.Spec.SourceSpec,
		},
	}

	if ready != nil {
		cs.Status = *makeContainerSourceStatus(ready)
	}
	return cs
}

func makeContainerSourceStatus(ready *corev1.ConditionStatus) *sourcesv1.ContainerSourceStatus {
	return &sourcesv1.ContainerSourceStatus{
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

func makeIntegrationSourceSpec(sink duckv1.Destination) sourcesv1alpha1.IntegrationSourceSpec {
	return sourcesv1alpha1.IntegrationSourceSpec{
		Timer: &sourcesv1alpha1.Timer{
			Period:  1000,
			Message: "Hallo",
		},
		SourceSpec: duckv1.SourceSpec{
			Sink: sink,
		},
	}
}
