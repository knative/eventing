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

package pingsource

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/adapter/mtping"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta2/pingsource"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	. "knative.dev/eventing/pkg/reconciler/testing"
	rtv1beta1 "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	rtv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1beta1",
		},
	}
	sinkDNS = "sink.mynamespace.svc." + network.GetClusterDomainName()
	sinkURI = apis.HTTP(sinkDNS)
)

const (
	sourceName      = "test-ping-source"
	sourceUID       = "1234"
	testNS          = "testnamespace"
	testSchedule    = "*/2 * * * *"
	testContentType = cloudevents.TextPlain
	testData        = "data"
	testDataBase64  = "ZGF0YQ==" // "data"

	sinkName   = "testsink"
	generation = 1
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
				rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1beta2.WithInitPingSourceConditions,
					rtv1beta2.WithPingSourceStatusObservedGeneration(generation),
					rtv1beta2.WithPingSourceSinkNotFound,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SinkNotFound",
					`Sink not found: {"ref":{"kind":"Channel","namespace":"testnamespace","name":"testsink","apiVersion":"messaging.knative.dev/v1beta1"}}`),
			},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1beta2.WithInitPingSourceConditions,
					rtv1beta2.WithPingSourceDeployed,
					rtv1beta2.WithPingSourceSink(sinkURI),
					rtv1beta2.WithPingSourceCloudEventAttributes,
					rtv1beta2.WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		}, {
			Name: "valid with dataBase64",
			Objects: []runtime.Object{
				rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
				),
				rtv1beta1.NewChannel(sinkName, testNS,
					rtv1beta1.WithInitChannelConditions,
					rtv1beta1.WithChannelAddress(sinkDNS),
				),
				makeAvailableMTAdapter(),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: rtv1beta2.NewPingSource(sourceName, testNS,
					rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink: sinkDest,
						},
					}),
					rtv1beta2.WithPingSource(sourceUID),
					rtv1beta2.WithPingSourceObjectMetaGeneration(generation),
					// Status Update:
					rtv1beta2.WithInitPingSourceConditions,
					rtv1beta2.WithPingSourceDeployed,
					rtv1beta2.WithPingSourceSink(sinkURI),
					rtv1beta2.WithPingSourceCloudEventAttributes,
					rtv1beta2.WithPingSourceStatusObservedGeneration(generation),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			kubeClientSet:    fakekubeclient.Get(ctx),
			pingLister:       listers.GetPingSourceV1beta2Lister(),
			deploymentLister: listers.GetDeploymentLister(),
			tracker:          tracker.New(func(types.NamespacedName) {}, 0),
		}
		r.sinkResolver = resolver.NewURIResolver(ctx, func(types.NamespacedName) {})

		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceV1beta2Lister(),
			controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func MakeMTAdapter() *appsv1.Deployment {
	args := resources.Args{
		NoShutdownAfter: mtping.GetNoShutDownAfterValue(),
		SinkTimeout:     adapter.GetSinkTimeout(nil),
	}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployments",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      mtadapterName,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: containerName,
							Env:  resources.MakeReceiveAdapterEnvVar(args),
						}}}}}}

}

func makeAvailableMTAdapter() *appsv1.Deployment {
	ma := MakeMTAdapter()
	WithDeploymentAvailable()(ma)
	return ma
}
