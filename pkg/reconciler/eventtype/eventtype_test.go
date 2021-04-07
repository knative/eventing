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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/eventtype"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/tracker"
)

const (
	testNS          = "test-namespace"
	eventTypeName   = "test-eventtype"
	eventTypeType   = "test-type"
	eventTypeBroker = "test-broker"
)

var (
	testKey         = fmt.Sprintf("%s/%s", testNS, eventTypeName)
	eventTypeSource = &apis.URL{
		Scheme: "http",
		Host:   "test-source",
	}
)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
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
	}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventTypeLister: listers.GetEventTypeLister(),
			brokerLister:    listers.GetBrokerLister(),
			tracker:         tracker.New(func(types.NamespacedName) {}, 0),
		}
		return eventtype.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetEventTypeLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}
