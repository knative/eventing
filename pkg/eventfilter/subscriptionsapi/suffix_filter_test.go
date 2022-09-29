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

	"knative.dev/eventing/pkg/eventfilter"
)

const (
	eventTypeSuffix      = ".knative.example"
	eventSourceSuffix    = "source"
	extensionValueSuffix = "extension-value"
)

func TestSuffixFilter(t *testing.T) {
	tests := map[string]struct {
		attribute string
		suffix    string
		event     *cloudevents.Event
		want      eventfilter.FilterResult
	}{
		"Missing attribute": {
			attribute: "some-other-attribute",
			suffix:    "wrong.suffix",
			want:      eventfilter.FailFilter,
		},
		"Wrong type suffix": {
			attribute: "type",
			suffix:    "wrong.suffix",
			want:      eventfilter.FailFilter,
		},
		"Wrong source suffix": {
			attribute: "source",
			suffix:    "wrong.suffix",
			want:      eventfilter.FailFilter,
		},
		"Wrong extension suffix": {
			attribute: extensionName,
			suffix:    "wrong.suffix",
			want:      eventfilter.FailFilter,
		},
		"Match type suffix": {
			attribute: "type",
			suffix:    eventTypeSuffix,
			want:      eventfilter.PassFilter,
		},
		"Match extension suffix": {
			attribute: extensionName,
			suffix:    extensionValueSuffix,
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
			f, err := NewSuffixFilter(map[string]string{
				tt.attribute: tt.suffix,
			})
			if err != nil {
				t.Errorf("error while creating suffix filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMultipleSuffixFilters(t *testing.T) {
	tests := map[string]struct {
		attributes []string
		values     []string
		event      *cloudevents.Event
		want       eventfilter.FilterResult
	}{
		"Wrong type and source": {
			attributes: []string{"type", "source"},
			values:     []string{"wrong.suffix", "wrong.suffix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and wrong source": {
			attributes: []string{"type", "source"},
			values:     []string{eventTypeSuffix, "wrong.suffix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and match source": {
			attributes: []string{"type", "source"},
			values:     []string{eventTypeSuffix, eventSourceSuffix},
			want:       eventfilter.PassFilter,
		},
		"Match type and missing extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventTypeSuffix, "attri.suffix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and match extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventTypeSuffix, extensionValueSuffix},
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
			f, err := NewSuffixFilter(map[string]string{
				tt.attributes[0]: tt.values[0],
				tt.attributes[1]: tt.values[1],
			})
			if err != nil {
				t.Errorf("error while creating suffix filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
