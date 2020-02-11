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

	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtype"

	"knative.dev/pkg/configmap"

	"knative.dev/pkg/tracker"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"
	. "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
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
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "EventType not found",
		Key:  testKey,
	}, {
		Name: "EventType being deleted",
		Key:  testKey,
		Objects: []runtime.Object{
			NewEventType(eventTypeName, testNS,
				WithInitEventTypeConditions,
				WithEventTypeDeletionTimestamp),
		},
	}, {
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
			Eventf(corev1.EventTypeWarning, "InternalError", `broker.eventing.knative.dev "test-broker" not found`),
		},
	}, {
		Name: "The status of Broker is False",
		Key:  testKey,
		Objects: []runtime.Object{
			NewEventType(eventTypeName, testNS,
				WithEventTypeType(eventTypeType),
				WithEventTypeSource(eventTypeSource),
				WithEventTypeBroker(eventTypeBroker),
			),
			NewBroker(eventTypeBroker, testNS,
				WithInitBrokerConditions,
				WithIngressFailed("DeploymentFailure", "inducing failure for create deployments"),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewEventType(eventTypeName, testNS,
				WithEventTypeType(eventTypeType),
				WithEventTypeSource(eventTypeSource),
				WithEventTypeBroker(eventTypeBroker),
				WithEventTypeBrokerExists,
				WithEventTypeBrokerFailed("DeploymentFailure", "inducing failure for create deployments"),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "EventTypeReconciled", `EventType reconciled: "test-namespace/test-eventtype"`),
		},
	}, {
		Name: "The status of Broker is Unknown",
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
				WithEventTypeBrokerUnknown("", ""),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "EventTypeReconciled", `EventType reconciled: "test-namespace/test-eventtype"`),
		},
	}, {
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
			Eventf(corev1.EventTypeNormal, "EventTypeReconciled", `EventType reconciled: "test-namespace/test-eventtype"`),
		},
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		b := reconciler.NewBase(ctx, controllerAgentName, cmw)
		r := &Reconciler{
			eventTypeLister: listers.GetEventTypeLister(),
			brokerLister:    listers.GetBrokerLister(),
			tracker:         tracker.New(func(types.NamespacedName) {}, 0),
		}
		return eventtype.NewReconciler(ctx, b.Logger, b.EventingClientSet, listers.GetEventTypeLister(), b.Recorder, r)
	},
		false,
		logger,
	))
}
