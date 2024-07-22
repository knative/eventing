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

	"knative.dev/eventing/pkg/reconciler/sugar/resources"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/resolver"
	"knative.dev/pkg/client/injection/ducks/duck/v1/kresource"
	"knative.dev/pkg/tracker"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta3/eventtype"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS          = "test-namespace"
	eventTypeName   = "test-eventtype"
	eventTypeType   = "test-type"
	eventTypeBroker = "test-broker"

	eventTypeChannel = "test-channel"
)

var (
	testKey         = fmt.Sprintf("%s/%s", testNS, eventTypeName)
	eventTypeSource = &apis.URL{
		Scheme: "http",
		Host:   "test-source",
	}
)

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
		Name: "Reference broker not found",
		Key:  testKey,
		Objects: []runtime.Object{
			NewEventType(eventTypeName, testNS,
				WithEventTypeType(eventTypeType),
				WithEventTypeSource(eventTypeSource),
				WithEventTypeSpecV1(),
				WithEventTypeEmptyID(),
				WithEventTypeReference(brokerReference(eventTypeBroker)),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewEventType(eventTypeName, testNS,
				WithEventTypeType(eventTypeType),
				WithEventTypeSource(eventTypeSource),
				WithEventTypeSpecV1(),
				WithEventTypeEmptyID(),
				WithEventTypeReference(brokerReference(eventTypeBroker)),
				WithInitEventTypeConditions,
				WithEventTypeResourceDoesNotExist,
			),
		}},

		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError failed to get object test-namespace/test-broker:", `brokers.eventing.knative.dev "test-broker" not found`),
		},
	},
		{
			Name: "Reference Channel not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithEventTypeReference(channelReference(eventTypeChannel)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithInitEventTypeConditions,
					WithEventTypeReference(channelReference(eventTypeChannel)),
					WithEventTypeResourceDoesNotExist,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError failed to get object test-namespace/test-channel:", `inmemorychannels.messaging.knative.dev "test-channel" not found`),
			},
		},
		{
			Name: "Reference broker found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithEventTypeReference(brokerReference(eventTypeBroker)),
				),
				resources.MakeBroker(testNS, eventTypeBroker),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithInitEventTypeConditions,
					WithEventTypeReference(brokerReference(eventTypeBroker)),
					WithEventTypeResourceExists,
				),
			}},
			WantErr: false,
		},
		{
			Name: "Reference channel found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithEventTypeReference(channelReference(eventTypeChannel)),
				),
				NewInMemoryChannel(eventTypeChannel, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithInitEventTypeConditions,
					WithEventTypeReference(channelReference(eventTypeChannel)),
					WithEventTypeResourceExists,
				),
			}},
			WantErr: false,
		}, {
			Name: "No reference set",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithInitEventTypeConditions,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeSpecV1(),
					WithEventTypeEmptyID(),
					WithEventTypeReferenceNotSet),
			}},
			WantErr: false,
		}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = kresource.WithDuck(ctx)
		r := &Reconciler{
			kReferenceResolver: resolver.NewKReferenceResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
		}
		return eventtype.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetEventTypeLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}

func brokerReference(brokerName string) *duckv1.KReference {
	return &duckv1.KReference{
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
		Name:       brokerName,
		Namespace:  testNS,
	}
}

func channelReference(channelName string) *duckv1.KReference {
	return &duckv1.KReference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Name:       channelName,
		Namespace:  testNS,
	}
}
