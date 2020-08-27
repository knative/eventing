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
	"sync"
	"testing"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testclient "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	adaptertesting "knative.dev/eventing/pkg/adapter/v2/test"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS               = "test-namespace"
	pingSourceName       = "test-pingsource"
	testSchedule         = "*/2 * * * *"
	testData             = "data"
	sinkName             = "mysink"
	defaultFinalizerName = "pingsources.sources.knative.dev"
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
					WithPingSourceV1A2CloudEventAttributes,
				),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "%s" finalizers`, pingSourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, pingSourceName, defaultFinalizerName),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, with finalizer",
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
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2Finalizers(defaultFinalizerName),
				),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, deleted with finalizer",
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
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2Finalizers(defaultFinalizerName),
					WithPingSourceV1A2Deleted,
				),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "%s" finalizers`, pingSourceName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, pingSourceName, ""),
			},
			WantErr: false,
		}, {
			Name: "valid schedule, deleted without finalizer",
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
					WithPingSourceV1A2CloudEventAttributes,
					WithPingSourceV1A2Deleted,
				),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	ce := adaptertesting.NewTestClient()

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			kubeClient:        testclient.NewSimpleClientset(),
			eventingClientSet: eventingclient.Get(ctx),
			pingsourceLister:  listers.GetPingSourceV1alpha2Lister(),
			cronRunner:        NewCronJobsRunner(ce, testclient.NewSimpleClientset(), logger),
			entryidMu:         sync.RWMutex{},
			entryids:          make(map[string]cron.EntryID),
		}
		return pingsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakeeventingclient.Get(ctx), listers.GetPingSourceV1alpha2Lister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))

}

func patchFinalizers(namespace, name string, finalizers string) clientgotesting.PatchActionImpl {
	fstr := ""
	if finalizers != "" {
		fstr = `"` + finalizers + `"`
	}
	return clientgotesting.PatchActionImpl{
		ActionImpl: clientgotesting.ActionImpl{
			Namespace:   namespace,
			Verb:        "patch",
			Resource:    schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1alpha2", Resource: "pingsources"},
			Subresource: "",
		},
		Name:      name,
		PatchType: "application/merge-patch+json",
		Patch:     []byte(`{"metadata":{"finalizers":[` + fstr + `],"resourceVersion":""}}`),
	}
}
