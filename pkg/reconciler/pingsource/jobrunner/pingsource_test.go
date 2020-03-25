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

package jobrunner

import (
	"context"
	"sync"
	"testing"

	"github.com/robfig/cron"
	"k8s.io/apimachinery/pkg/runtime"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/source"
)

const (
	testNS         = "test-namespace"
	pingSourceName = "test-pingsource"
	testSchedule   = "*/2 * * * *"
	testData       = "data"
	sinkName       = "mysink"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNS,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}

	sinkURI, _ = apis.ParseURL("https://" + sinkName)
)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount += 1
	return nil
}

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
				NewPingSourceV1Alpha2(pingSourceName, testNS,
					WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
						Schedule: testSchedule,
						JsonData: testData,
						SourceSpec: duckv1.SourceSpec{
							Sink:                sinkDest,
							CloudEventOverrides: nil,
						},
					}),
					WithInitPingSourceV1A2Conditions,
					WithValidPingSourceV1A2Schedule,
					WithPingSourceV1A2Deployed,
					WithPingSourceV1A2Sink(sinkURI),
					WithPingSourceV1A2EventType,
				),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	reporter := &mockReporter{}
	ce := kncetesting.NewTestClient()

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			pingsourceLister: listers.GetPingSourceV1alpha2Lister(),
			cronRunner:       NewCronJobsRunner(ce, reporter, logger),
			entryidMu:        sync.Mutex{},
			entryids:         make(map[string]cron.EntryID),
		}
		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceV1alpha2Lister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))

}
