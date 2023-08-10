/*
Copyright 2023 The Knative Authors

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
	"reflect"
	"testing"

	v2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/eventing/v1beta2"
	"knative.dev/eventing/pkg/apis/feature"
	fakeeventingclientset "knative.dev/eventing/pkg/client/clientset/versioned/fake"
	reconcilertestingv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestEventTypeAutoHandler_AutoCreateEventType(t *testing.T) {
	testCases := []struct {
		name              string
		featureFlag       string
		addressable       *duckv1.KReference
		events            []v2.Event
		expectedEventType []v1beta2.EventType
		expectedError     error
	}{
		{
			name:        "With 1 broker and 1 type",
			featureFlag: "enabled",
			addressable: &duckv1.KReference{
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
				Namespace:  "default",
				Name:       "broker"},
			events: []v2.Event{initEvent("")},
			expectedEventType: []v1beta2.EventType{
				initEventTypeObject(),
				initEventTypeObject()},
			expectedError: nil,
		},
		{
			name:        "With 1 broker and multiple types",
			featureFlag: "enabled",
			addressable: &duckv1.KReference{
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
				Namespace:  "default",
				Name:       "broker"},
			events: []v2.Event{
				initEvent("foo.type"),
				initEvent("bar.type")},
			expectedEventType: []v1beta2.EventType{
				initEventTypeObject(),
				initEventTypeObject()},
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		ctx := context.TODO()
		eventtypes := make([]runtime.Object, 0, 10)
		listers := reconcilertestingv1beta2.NewListers(eventtypes)
		eventingClient := fakeeventingclientset.NewSimpleClientset()
		logger := zap.NewNop()

		handler := &EventTypeAutoHandler{
			EventTypeLister: listers.GetEventTypeLister(),
			EventingClient:  eventingClient.EventingV1beta2(),
			FeatureStore:    initFeatureStore(t, tc.featureFlag),
			Logger:          logger,
		}

		ownerUID := types.UID("owner-uid")

		for i, event := range tc.events {

			err := handler.AutoCreateEventType(ctx, &event, tc.addressable, ownerUID)
			if err != nil {
				if tc.expectedError == err {
					t.Errorf("test case '%s', expected '%s', got '%s'", tc.name, tc.expectedError, err)
				} else {
					t.Error(err)
				}
			}

			etName := generateEventTypeName(tc.addressable.Name, tc.addressable.Namespace, event.Type(), event.Source())
			et, err := eventingClient.EventingV1beta2().EventTypes(tc.addressable.Namespace).Get(ctx, etName, metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(et.Spec.Reference, tc.expectedEventType[i].Spec.Reference) {
				t.Errorf("test case '%s', expected '%s', got '%s'", tc.name, tc.expectedEventType[i].Spec.Reference, et.Spec.Reference)
			}
		}
	}

}

func TestEventTypeAutoHandler_GenerateEventTypeName(t *testing.T) {
	testCases := []struct {
		name         string
		namespace    string
		eventType    string
		eventSource  string
		expectedName string
	}{
		{
			name:         "example",
			namespace:    "default",
			eventType:    "events.type",
			eventSource:  "events.source",
			expectedName: "et-example-eaed2996d719048dfae11b607d68b7ad",
		},
		{
			name:         "EXAMPLE",
			namespace:    "default",
			eventType:    "events.type",
			eventSource:  "events.source",
			expectedName: "et-example-f4d9f28cdbe397fe92eb5899e7294121",
		},
		{
			name:         "",
			namespace:    "default",
			eventType:    "events.type",
			eventSource:  "events.source",
			expectedName: "et--72f2dad9d914dcd0445cae74ba99d15c",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generateEventTypeName(tc.name, tc.namespace, tc.eventType, tc.eventSource)

			if result != tc.expectedName {
				t.Errorf("test case '%s', expected '%s', got '%s'", tc.name, tc.expectedName, result)
			}
		})
	}
}

func TestEventTypeAutoHandler_GenerateEventTypeNameUnique(t *testing.T) {
	type eventTypeMock struct {
		name        string
		namespace   string
		eventType   string
		eventSource string
	}

	testCases := []struct {
		name           string
		eventTypeA     eventTypeMock
		eventTypeB     eventTypeMock
		expectedUnique bool
	}{
		{
			name: "Long equal name of addressable",
			eventTypeA: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.A",
			},
			eventTypeB: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeB",
				eventSource: "events.source.B",
			},
			expectedUnique: true,
		},
		{
			name: "Long equal name & same source",
			eventTypeA: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.A",
			},
			eventTypeB: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeB",
				eventSource: "events.source.A",
			},
			expectedUnique: true,
		},
		{
			name: "Long name with one char diff",
			eventTypeA: eventTypeMock{
				name:        "long-addressable-nameA",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.A",
			},
			eventTypeB: eventTypeMock{
				name:        "long-addressable-nameB",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.B",
			},
			expectedUnique: true,
		},
		{
			name: "All same input params are not unique",
			eventTypeA: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.A",
			},
			eventTypeB: eventTypeMock{
				name:        "long-addressable-name",
				namespace:   "default",
				eventType:   "com.example.organization.events.typeA",
				eventSource: "events.source.A",
			},
			expectedUnique: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultA := generateEventTypeName(tc.eventTypeA.name, tc.eventTypeA.namespace, tc.eventTypeA.eventType, tc.eventTypeA.eventSource)
			resultB := generateEventTypeName(tc.eventTypeB.name, tc.eventTypeB.namespace, tc.eventTypeB.eventType, tc.eventTypeB.eventSource)

			if tc.expectedUnique {
				if resultA == resultB {
					t.Errorf("test case '%s', generated name '%s' x '%s' must be unique", tc.name, resultA, resultB)
				}
			} else {
				if resultA != resultB {
					t.Errorf("test case '%s', generated name '%s' x '%s' must be equal", tc.name, resultA, resultB)
				}
			}
		})
	}
}

func initFeatureStore(t *testing.T, enabled string) *feature.Store {
	featureStore := feature.NewStore(logtesting.TestLogger(t))
	cm := resources.ConfigMap(
		"config-features",
		"default",
		map[string]string{feature.EvenTypeAutoCreate: enabled},
	)
	featureStore.OnConfigChanged(cm)
	return featureStore
}

func initEvent(eventType string) v2.Event {
	e := event.New()
	e.SetType(eventType)
	if eventType == "" {
		e.SetType("test.Type")
	}
	e.SetSource("test.source")
	return e
}

func initEventTypeObject() v1beta2.EventType {
	return v1beta2.EventType{
		Spec: v1beta2.EventTypeSpec{
			Reference: &duckv1.KReference{
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
				Namespace:  "default",
				Name:       "broker"},
		},
	}
}
