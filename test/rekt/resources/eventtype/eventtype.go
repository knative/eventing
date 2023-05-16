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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	eventingv1beta2 "knative.dev/eventing/pkg/apis/eventing/v1beta2"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

type EventType struct {
	Name       string
	EventTypes func(etl eventingv1beta2.EventTypeList) (bool, error)
}

func WaitForEventType(eventtype EventType, timing ...time.Duration) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		interval, timeout := k8s.PollTimings(ctx, timing)
		var lastErr error
		var lastEtl *eventingv1beta2.EventTypeList
		err := wait.PollImmediate(interval, timeout, func() (done bool, err error) {
			etl, err := eventingclient.Get(ctx).
				EventingV1beta2().
				EventTypes(env.Namespace()).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			lastEtl = etl
			return eventtype.EventTypes(*etl)
		})
		if err != nil {
			t.Fatalf("failed to verify eventtype %s %v: %v\n%+v\n", eventtype.Name, err, lastErr, lastEtl)
		}

	}
}

func AssertPresent(expectedCeTypes sets.String) EventType {
	return EventType{
		Name: "test eventtypes match or not",
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			eventtypesCount := 0
			for _, et := range etl.Items {
				if expectedCeTypes.Has(et.Spec.Type) {
					eventtypesCount++
				}
			}
			return (eventtypesCount == len(etl.Items)), nil
		},
	}

}
