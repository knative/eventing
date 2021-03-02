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

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta2/pingsource"
	. "knative.dev/eventing/pkg/reconciler/testing"
	rttestingv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
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
			APIVersion: "messaging.knative.dev/v1beta1",
		},
	}

	sinkURI, _ = apis.ParseURL("https://" + sinkName)
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
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule without contentType, data and dataBase64",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule: testSchedule,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule with dataBase64",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						DataBase64:  testDataBase64,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, with finalizer",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, deleted with finalizer",
			Key:  pingsourceKey,
			Objects: []runtime.Object{
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
					rttestingv1beta2.WithPingSourceDeleted,
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, deleted without finalizer",
			Key:  "a/a",
			Objects: []runtime.Object{
				rttestingv1beta2.NewPingSource(pingSourceName, testNS,
					rttestingv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
						Schedule:    testSchedule,
						ContentType: testContentType,
						Data:        testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					rttestingv1beta2.WithInitPingSourceConditions,
					rttestingv1beta2.WithPingSourceDeployed,
					rttestingv1beta2.WithPingSourceSink(sinkURI),
					rttestingv1beta2.WithPingSourceCloudEventAttributes,
					rttestingv1beta2.WithPingSourceDeleted,
				),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{mtadapter: testAdapter{}}
		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceV1beta2Lister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))

}

func TestReconciler_deleteFunc(t *testing.T) {
	pingsourceKey := testNS + "/" + pingSourceName
	adapter := &testAdapter{}
	p := rttestingv1beta2.NewPingSource(pingSourceName, testNS)
	r := &Reconciler{mtadapter: adapter}
	r.deleteFunc(p)
	if _, ok := removePingsource[pingsourceKey]; !ok {
		t.Errorf("Got error when call deleteFunc")
	}
}
