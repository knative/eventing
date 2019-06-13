/*
Copyright 2019 The Knative Authors

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

package eventtype

import (
	"context"
	"fmt"
	"testing"

	"github.com/knative/pkg/configmap"

	"github.com/knative/pkg/tracker"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS          = "test-namespace"
	eventTypeName   = "test-eventtype"
	eventTypeType   = "test-type"
	eventTypeBroker = "test-broker"
	eventTypeSource = "/test-source"
)

var (
	trueVal = true

	testKey = fmt.Sprintf("%s/%s", testNS, eventTypeName)
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "EventType not found",
			Key:  testKey,
		},
		{
			Name: "EventType being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithInitEventTypeConditions,
					WithEventTypeDeletionTimestamp),
			},
		},
		{
			Name: "Broker not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithInitEventTypeConditions,
					WithEventTypeBrokerDoesNotExist,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, eventTypeReconcileFailed, "EventType reconcile error: broker.eventing.knative.dev %q not found", eventTypeBroker),
			},
		},
		{
			Name: "Broker not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
				NewBroker(eventTypeBroker, testNS,
					WithInitBrokerConditions,
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithEventTypeBrokerExists,
					WithEventTypeBrokerNotReady,
				),
			}},
		},
		{
			Name: "Successful reconcile, became ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
				NewBroker(eventTypeBroker, testNS,
					WithBrokerReady,
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithEventTypeBrokerExists,
					WithEventTypeBrokerReady,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, eventTypeReadinessChanged, "EventType %q became ready", eventTypeName),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			eventTypeLister: listers.GetEventTypeLister(),
			brokerLister:    listers.GetBrokerLister(),
			tracker:         tracker.New(func(string) {}, 0),
		}
	},
		false,
	))
}
