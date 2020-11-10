package attributes

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
	broker "knative.dev/eventing/pkg/mtbroker"
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
			filter: attributesFilter("some-other-type", ""),
			want:   eventfilter.FailFilter,
		},
		"Wrong type with attribs": {
			filter: attributesFilter("some-other-type", ""),
			want:   eventfilter.FailFilter,
		},
		"Wrong source": {
			filter: attributesFilter("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Wrong source with attribs": {
			filter: attributesFilter("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Wrong extension": {
			filter: attributesFilter("", "some-other-source"),
			want:   eventfilter.FailFilter,
		},
		"Any": {
			filter: attributesFilter("", ""),
			want:   eventfilter.PassFilter,
		},
		"Specific": {
			filter: attributesFilter(eventType, eventSource),
			want:   eventfilter.PassFilter,
		},
		"Extension with attribs": {
			filter: attributesWithExtensionFilter(eventType, eventSource, extensionValue),
			event:  makeEventWithExtension(extensionName, extensionValue),
			want:   eventfilter.PassFilter,
		},
		"Any with attribs - Arrival extension": {
			filter: attributesFilter("", ""),
			event:  makeEventWithExtension(broker.EventArrivalTime, "2019-08-26T23:38:17.834384404Z"),
			want:   eventfilter.PassFilter,
		},
		"Wrong Extension with attribs": {
			filter: attributesWithExtensionFilter(eventType, eventSource, "some-other-extension-value"),
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

func attributesFilter(t, s string) map[string]string {
	return map[string]string{
		"type":   t,
		"source": s,
	}
}

func attributesWithExtensionFilter(t, s, e string) map[string]string {
	return map[string]string{
		"type":        t,
		"source":      s,
		extensionName: e,
	}
}
