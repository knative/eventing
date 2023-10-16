/*
Copyright 2020 The Knative Authors

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

package mtping

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	rtv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
)

const (
	testNS          = "test-namespace"
	pingSourceName  = "test-pingsource"
	testSchedule    = "*/2 * * * *"
	testContentType = cloudevents.TextPlain
	testData        = "data"
	testDataBase64  = "ZGF0YQ=="
	sinkName        = "mysink"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}

	sinkURI, _ = apis.ParseURL("https://" + sinkName)
	sinkAddr   = &duckv1.Addressable{
		URL: sinkURI,
	}
)

func TestAllCases(t *testing.T) {
	pingsourceKey := testNS + "/" + pingSourceName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "valid schedule",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceSynchronized",
					`PingSource adapter is synchronized`),
			},
		}, {
			Name: "valid schedule without contentType, data and dataBase64",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule: testSchedule,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceSynchronized",
					`PingSource adapter is synchronized`),
			},
		}, {
			Name: "valid schedule with dataBase64",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceSynchronized",
					`PingSource adapter is synchronized`),
			},
		}, {
			Name: "valid schedule, with finalizer",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "PingSourceSynchronized",
					`PingSource adapter is synchronized`),
			},
		}, {
			Name: "valid schedule, deleted with finalizer",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceDeleted,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, deleted without finalizer",
			Key:  "a/a",
			Objects: []runtime.Object{
				rtv1.NewPingSource(pingSourceName, testNS,
					rtv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rtv1.WithInitPingSourceConditions,
					rtv1.WithPingSourceDeployed,
					rtv1.WithPingSourceSink(sinkAddr),
					rtv1.WithPingSourceCloudEventAttributes,
					rtv1.WithPingSourceDeleted,
					rtv1.WithPingSourceOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)

	table.Test(t, rtv1.MakeFactory(func(ctx context.Context, listers *rtv1.Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{mtadapter: testAdapter{}}
		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))

}

func TestReconciler_deleteFunc(t *testing.T) {
	pingsourceKey := testNS + "/" + pingSourceName
	adapter := &testAdapter{}
	p := rtv1.NewPingSource(pingSourceName, testNS)
	r := &Reconciler{mtadapter: adapter}
	r.deleteFunc(p)
	if _, ok := removePingsource[pingsourceKey]; !ok {
		t.Errorf("Got error when call deleteFunc")
	}
}
