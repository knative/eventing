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
	eventTypePrefix      = "dev.knative"
	eventSourcePrefix    = "knative"
	extensionValuePrefix = "my-extension"
)

func TestPrefixFilter(t *testing.T) {
	tests := map[string]struct {
		attribute string
		prefix    string
		event     *cloudevents.Event
		want      eventfilter.FilterResult
	}{
		"Missing attribute": {
			attribute: "some-other-attribute",
			prefix:    "wrong.prefix",
			want:      eventfilter.FailFilter,
		},
		"Wrong type prefix": {
			attribute: "type",
			prefix:    "wrong.prefix",
			want:      eventfilter.FailFilter,
		},
		"Wrong source prefix": {
			attribute: "source",
			prefix:    "wrong.prefix",
			want:      eventfilter.FailFilter,
		},
		"Wrong extension prefix": {
			attribute: extensionName,
			prefix:    "wrong.prefix",
			want:      eventfilter.FailFilter,
		},
		"Match type prefix": {
			attribute: "type",
			prefix:    eventTypePrefix,
			want:      eventfilter.PassFilter,
		},
		"Match extension prefix": {
			attribute: extensionName,
			prefix:    extensionValuePrefix,
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
			f, err := NewPrefixFilter(map[string]string{
				tt.attribute: tt.prefix,
			})
			if err != nil {
				t.Errorf("error while creating prefix filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMultiplePrefixFilters(t *testing.T) {
	tests := map[string]struct {
		attributes []string
		values     []string
		event      *cloudevents.Event
		want       eventfilter.FilterResult
	}{
		"Wrong type and source": {
			attributes: []string{"type", "source"},
			values:     []string{"wrong.prefix", "wrong.prefix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and wrong source": {
			attributes: []string{"type", "source"},
			values:     []string{eventTypePrefix, "wrong.prefix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and match source": {
			attributes: []string{"type", "source"},
			values:     []string{eventTypePrefix, eventSourcePrefix},
			want:       eventfilter.PassFilter,
		},
		"Match type and missing extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventTypePrefix, "attri.prefix"},
			want:       eventfilter.FailFilter,
		},
		"Match type and match extension": {
			attributes: []string{"type", extensionName},
			values:     []string{eventTypePrefix, extensionValuePrefix},
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
			f, err := NewPrefixFilter(map[string]string{
				tt.attributes[0]: tt.values[0],
				tt.attributes[1]: tt.values[1],
			})
			if err != nil {
				t.Errorf("error while creating prefix filter %v", err)
			} else {
				if got := f.Filter(context.TODO(), *e); got != tt.want {
					t.Errorf("Filter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
