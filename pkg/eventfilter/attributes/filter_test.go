/*
Copyright 2020 The Knative Authors

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

package attributes

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	broker "knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/eventfilter"
)

const (
	eventType      = `com.example.someevent`
	eventSource    = `/mycontext`
	extensionName  = `myextension`
	extensionValue = `my-extension-value`
)

func TestAttributesFilter_Filter(t *testing.T) {
	tests := map[string]struct {
		filter map[string]string
		event  *cloudevents.Event
		want   eventfilter.FilterResult
	}{
		"Wrong type": {
			filter: attributes("some-other-type", ""),
			want:   eventfilter.FailFilter,
		},
		"Wrong type with attribs": {
			filter: attributes("some-other-type", ""),
			want:   eventfilter.FailFilter,
		},
		"Wrong source": {
			filter: attributes("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Wrong source with attribs": {
			filter: attributes("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Wrong extension": {
			filter: attributes("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Any": {
			filter: attributes("", ""),
			want:   eventfilter.PassFilter,
		},
		"Specific": {
			filter: attributes(eventType, eventSource),
			want:   eventfilter.PassFilter,
		},
		"Extension with attribs": {
			filter: attributesWithExtension(eventType, eventSource, extensionValue),
			event:  makeEventWithExtension(extensionName, extensionValue),
			want:   eventfilter.PassFilter,
		},
		"Any with attribs - Arrival extension": {
			filter: attributes("", ""),
			event:  makeEventWithExtension(broker.EventArrivalTime, "2019-08-26T23:38:17.834384404Z"),
			want:   eventfilter.PassFilter,
		},
		"Wrong Extension with attribs": {
			filter: attributesWithExtension(eventType, eventSource, "some-other-extension-value"),
			event:  makeEventWithExtension(extensionName, extensionValue),
			want:   eventfilter.FailFilter,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.event
			if e == nil {
				e = makeEvent()
			}

			if got := NewAttributesFilter(tt.filter).Filter(context.TODO(), *e); got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeEvent() *cloudevents.Event {
	e := cloudevents.NewEvent()
	e.SetType(eventType)
	e.SetSource(eventSource)
	e.SetID("1234")
	return &e
}

func makeEventWithExtension(extName, extValue string) *cloudevents.Event {
	e := makeEvent()
	e.SetExtension(extName, extValue)
	return e
}

func attributes(t, s string) map[string]string {
	return map[string]string{
		"type":   t,
		"source": s,
	}
}

func attributesWithExtension(t, s, e string) map[string]string {
	return map[string]string{
		"type":        t,
		"source":      s,
		extensionName: e,
	}
}
