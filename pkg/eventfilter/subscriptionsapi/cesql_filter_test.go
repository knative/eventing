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

package subscriptionsapi

import (
	"context"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
)

func TestCESQLFilter(t *testing.T) {
	tests := map[string]struct {
		expression string
		event      *cloudevents.Event
		want       eventfilter.FilterResult
	}{
		"Missing expression": {
			expression: "",
			want:       eventfilter.NoFilter,
		},
		"Wrong type": {
			expression: fmt.Sprintf("type = '%s'", "wrong.type"),
			want:       eventfilter.FailFilter,
		},
		"Wrong extension prefix": {
			expression: fmt.Sprintf("%s = '%s'", extensionName, "wrong.extension"),
			want:       eventfilter.FailFilter,
		},
		"Match type": {
			expression: fmt.Sprintf("type = '%s'", eventType),
			want:       eventfilter.PassFilter,
		},
		"Match extension": {
			expression: fmt.Sprintf("%s = '%s'", extensionName, extensionValue),
			event:      makeEventWithExtension(extensionName, extensionValue),
			want:       eventfilter.PassFilter,
		},
		"Match numeric extension": {
			expression: fmt.Sprintf("%s < 30", eventTTLName),
			want:       eventfilter.PassFilter,
		},
		"Match type OR Source": {
			expression: fmt.Sprintf("(type = '%s') OR (source = '%s')", eventType, "some-other-source"),
			want:       eventfilter.PassFilter,
		},
		"Missing attribute less than comparison": {
			expression: fmt.Sprintf("missingattribute < %d", 5),
			want:       eventfilter.FailFilter,
		},
		"Missing attribute equals comparison": {
			expression: fmt.Sprintf("missingattribute = %s", "missinsvalue"),
			want:       eventfilter.FailFilter,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.event
			if e == nil {
				e = makeEvent()
			}
			f, err := NewCESQLFilter(tt.expression)
			if err != nil {
				t.Fatalf("Error inistanciating CESQL filter. %v", err)
			}
			if got := f.Filter(context.TODO(), *e); got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
