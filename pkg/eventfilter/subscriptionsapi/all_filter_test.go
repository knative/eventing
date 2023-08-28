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

	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/eventfilter"
)

const (
	eventType      = "dev.knative.example"
	eventSource    = "knativesource"
	eventID        = "1234"
	extensionName  = "myextension"
	extensionValue = "my-extension-value"
	eventTTLName   = "knativebrokerttl"
	eventTTLValue  = 20
)

type passFilter struct{}

func (*passFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	return eventfilter.PassFilter
}

func (*passFilter) Cleanup() {}

type failFilter struct{}

func (*failFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	return eventfilter.FailFilter
}

func (*failFilter) Cleanup() {}

func TestAllFilter_Flat(t *testing.T) {
	tests := map[string]struct {
		filter eventfilter.Filter
		want   eventfilter.FilterResult
	}{
		"Empty": {
			filter: NewAllFilter(),
			want:   eventfilter.NoFilter,
		},
		"One Pass": {
			filter: NewAllFilter(&passFilter{}),
			want:   eventfilter.PassFilter,
		},
		"One Fail": {
			filter: NewAllFilter(&failFilter{}),
			want:   eventfilter.FailFilter,
		},
		"Pass and Fail": {
			filter: NewAllFilter(&passFilter{}, &failFilter{}),
			want:   eventfilter.FailFilter,
		},
		"Pass and Pass": {
			filter: NewAllFilter(&passFilter{}, &passFilter{}),
			want:   eventfilter.PassFilter,
		},
		"Fail and Fail": {
			filter: NewAllFilter(&failFilter{}, &failFilter{}),
			want:   eventfilter.FailFilter,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := makeEvent()
			if got := tc.filter.Filter(context.TODO(), *e); got != tc.want {
				t.Errorf("Filter() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAllFilter_WithNestedAll(t *testing.T) {
	tests := map[string]struct {
		filter eventfilter.Filter
		want   eventfilter.FilterResult
	}{
		"All(All(Pass))": {
			filter: NewAllFilter(
				NewAllFilter(&passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"All(All(Fail))": {
			filter: NewAllFilter(
				NewAllFilter(&failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Pass,All(Fail))": {
			filter: NewAllFilter(&passFilter{},
				NewAllFilter(&failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Pass,All(Pass))": {
			filter: NewAllFilter(&passFilter{},
				NewAllFilter(&passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"All(Fail,All(Fail))": {
			filter: NewAllFilter(&failFilter{},
				NewAllFilter(&failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Fail,All(Pass))": {
			filter: NewAllFilter(&failFilter{},
				NewAllFilter(&passFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Pass,All(Pass,Fail))": {
			filter: NewAllFilter(&passFilter{},
				NewAllFilter(&passFilter{}, &failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Pass,All(Pass,Pass))": {
			filter: NewAllFilter(&passFilter{},
				NewAllFilter(&passFilter{}, &passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"All(Pass,All(Fail,Fail))": {
			filter: NewAllFilter(&passFilter{},
				NewAllFilter(&failFilter{}, &failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Fail,All(Pass,Fail))": {
			filter: NewAllFilter(&failFilter{},
				NewAllFilter(&passFilter{}, &failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Fail,All(Pass,Pass))": {
			filter: NewAllFilter(&failFilter{},
				NewAllFilter(&passFilter{}, &passFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"All(Fail,All(Fail,Fail))": {
			filter: NewAllFilter(&failFilter{},
				NewAllFilter(&failFilter{}, &failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := makeEvent()
			if got := tc.filter.Filter(context.TODO(), *e); got != tc.want {
				t.Errorf("Filter() = %s, want %s", got, tc.want)
			}
		})
	}
}

func makeEvent() *cloudevents.Event {
	e := cloudevents.NewEvent()
	e.SetType(eventType)
	e.SetSource(eventSource)
	e.SetID(eventID)
	_ = broker.SetTTL(e.Context, eventTTLValue)
	return &e
}
