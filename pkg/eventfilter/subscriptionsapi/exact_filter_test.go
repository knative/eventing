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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/pkg/eventfilter"
)

func TestExactMatchFilter(t *testing.T) {
	tests := map[string]struct {
		attribute string
		value     string
		event     *cloudevents.Event
		want      eventfilter.FilterResult
	}{
		"Wrong type": {
			attribute: "type",
			value:     "some-other-type",
			want:      eventfilter.FailFilter,
		},
		"Wrong source": {
			attribute: "source",
			value:     "some-other-source",
			want:      eventfilter.FailFilter,
		},
		"Wrong extension": {
			attribute: extensionName,
			value:     "some-other-extension",
			want:      eventfilter.FailFilter,
		},
		"Match type": {
			attribute: "type",
			value:     eventType,
			want:      eventfilter.PassFilter,
		},
		"Match extension": {
			attribute: extensionName,
			value:     extensionValue,
			event:     makeEventWithExtension(extensionName, extensionValue),
			want:      eventfilter.PassFilter,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.event
			if e == nil {
				e = makeEvent()
			}
			f, err := NewExactFilter(map[string]string{
				tt.attribute: tt.value,
			})
			if err != nil {
				t.Errorf("error while creating exact filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMultipleExactMatchFilters(t *testing.T) {
	tests := map[string]struct {
		attributes []string
		values     []string
		event      *cloudevents.Event
		want       eventfilter.FilterResult
	}{
		"Wrong type and source": {
			attributes: []string{"type", "source"},
			values:     []string{"some-other-type", "some-other-source"},
			want:       eventfilter.FailFilter,
		},
		"Match type and wrong source": {
			attributes: []string{"type", "source"},
			values:     []string{eventType, "some-other-source"},
			want:       eventfilter.FailFilter,
		},
		"Match type and match source": {
			attributes: []string{"type", "source"},
			values:     []string{eventType, eventSource},
			want:       eventfilter.PassFilter,
		},
		"Match type and miss extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventType, extensionValue},
			want:       eventfilter.FailFilter,
		},
		"Match type and match extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventType, extensionValue},
			event:      makeEventWithExtension(extensionName, extensionValue),
			want:       eventfilter.PassFilter,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.event
			if e == nil {
				e = makeEvent()
			}
			f, err := NewExactFilter(map[string]string{
				tt.attributes[0]: tt.values[0],
				tt.attributes[1]: tt.values[1],
			})
			if err != nil {
				t.Errorf("error while creating exact filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func makeEventWithExtension(extName, extValue string) *cloudevents.Event {
	e := makeEvent()
	e.SetExtension(extName, extValue)
	return e
}

func TestExactTimeMatchIsStable(t *testing.T) {
	event := cetest.FullEvent()
	time, err := event.Time().MarshalText()
	if err != nil {
		t.Fatalf("error while marshalling time to text: %v", err)
	}
	f, err := NewExactFilter(map[string]string{
		"time": string(time),
	})
	if err != nil {
		t.Fatalf("failed to create filter")
	}
	// the filter should consistently pass here, let's check that it does
	failCount := 0
	for i := 0; i < 1000; i++ {
		if got := f.Filter(context.TODO(), event); got != eventfilter.PassFilter {
			failCount += 1
		}
	}
	if failCount != 0 {
		t.Errorf("exact filter match on time should be consistent, failed %d / 10,000 times for same event", failCount)
	}

}
