/*
Copyright 2018 The Knative Authors.

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

package testing

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	fakedynamicclientset "k8s.io/client-go/dynamic/fake"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	//ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
)

const (
	// maxEventBufferSize is the estimated max number of event notifications that
	// can be buffered during reconciliation.
	maxEventBufferSize = 10
)

// Ctor functions create a k8s controller with given params.
type Ctor func(*Listers, reconciler.Options) controller.Reconciler

// MakeFactory creates a reconciler factory with fake clients and controller created by `ctor`.
func MakeFactory(ctor Ctor) Factory {
	return func(t *testing.T, r *TableRow) (controller.Reconciler, ActionRecorderList, EventList, *FakeStatsReporter) {
		ls := NewListers(r.Objects)

		kubeClient := fakekubeclientset.NewSimpleClientset(ls.GetKubeObjects()...)
		client := fakeclientset.NewSimpleClientset(ls.GetServingObjects()...)

		dynamicClient := fakedynamicclientset.NewSimpleDynamicClient(runtime.NewScheme(), ls.GetBuildObjects()...)
		eventRecorder := record.NewFakeRecorder(maxEventBufferSize)
		statsReporter := &FakeStatsReporter{}

		PrependGenerateNameReactor(&client.Fake)
		PrependGenerateNameReactor(&dynamicClient.Fake)

		// Set up our Controller from the fakes.
		c := ctor(&ls, reconciler.Options{
			KubeClientSet:     kubeClient,
			DynamicClientSet:  dynamicClient,
			EventingClientSet: client,
			Recorder:          eventRecorder,
			//StatsReporter:    statsReporter,
			Logger: TestLogger(t),
		})

		for _, reactor := range r.WithReactors {
			kubeClient.PrependReactor("*", "*", reactor)
			client.PrependReactor("*", "*", reactor)
			dynamicClient.PrependReactor("*", "*", reactor)
		}

		// Validate all Create operations through the eventing client.
		client.PrependReactor("get", "*", ValidateGets)
		client.PrependReactor("create", "*", ValidateCreates)
		client.PrependReactor("update", "*", ValidateUpdates)

		actionRecorderList := ActionRecorderList{dynamicClient, client, kubeClient}
		eventList := EventList{Recorder: eventRecorder}

		return c, actionRecorderList, eventList, statsReporter
	}
}
