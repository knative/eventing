/*
Copyright 2022 The Knative Authors

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	eventingv1beta2 "knative.dev/eventing/pkg/apis/eventing/v1beta2"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
)

type EventType struct {
	Name       string
	EventTypes func(etl eventingv1beta2.EventTypeList) (bool, error)
}

func (et EventType) And(eventType EventType) EventType {
	return EventType{
		Name: fmt.Sprintf("%s and %s", et.Name, eventType.Name),
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			v, err := et.EventTypes(etl)
			if err != nil || !v {
				return v, err
			}
			return eventType.EventTypes(etl)
		},
	}
}

func WaitForEventType(eventtypes ...EventType) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		interval, timeout := k8s.PollTimings(ctx, nil)
		var lastErr error
		var lastEtl *eventingv1beta2.EventTypeList
		eventType := eventtypes[0] // It's fine to panic when is empty
		for _, et := range eventtypes[1:] {
			eventType = eventType.And(et)
		}
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (done bool, err error) {
			etl, err := eventingclient.Get(ctx).
				EventingV1beta2().
				EventTypes(env.Namespace()).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			lastEtl = etl
			return eventType.EventTypes(*etl)
		})
		if err != nil {
			t.Fatalf("failed to verify eventtype %s %v: %v\n%+v\n", eventType.Name, err, lastErr, lastEtl)
		}

	}
}

func AssertPresent(expectedCeTypes sets.Set[string]) EventType {
	return EventType{
		Name: "test expected EventTypes",
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			// Clone the expectedCeTypes
			clonedExpectedCeTypes := expectedCeTypes.Clone()
			for _, et := range etl.Items {
				clonedExpectedCeTypes.Delete(et.Spec.Type) // remove from the cloned set
			}
			return clonedExpectedCeTypes.Len() == 0, nil
		},
	}
}

func AssertReady(expectedCeTypes sets.Set[string]) EventType {
	return EventType{
		Name: "test EventTypes ready",
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			for _, et := range etl.Items {
				if expectedCeTypes.Has(et.Spec.Type) && !et.Status.IsReady() {
					return false, nil
				}
			}
			return true, nil
		},
	}
}

func AssertExactPresent(expectedCeTypes sets.Set[string]) EventType {
	return EventType{
		Name: "test eventtypes match or not",
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			// Clone the expectedCeTypes
			clonedExpectedCeTypes := expectedCeTypes.Clone()
			for _, et := range etl.Items {
				if !clonedExpectedCeTypes.Has(et.Spec.Type) {
					return false, nil
				}
				clonedExpectedCeTypes.Delete(et.Spec.Type) // remove from the cloned set
			}
			return clonedExpectedCeTypes.Len() == 0, nil
		},
	}
}
