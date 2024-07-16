/*
Copyright 2021 The Knative Authors

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

package knconf

import (
	"context"
	"sort"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/go-cmp/cmp"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

type EventPattern struct {
	Success  []bool
	Interval []uint
}

func ReceivedEventsFromProber(ctx context.Context, prober *eventshub.EventProber, prefix string) []conformanceevent.Event {
	events := prober.ReceivedOrRejectedBy(ctx, prefix)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	ret := make([]conformanceevent.Event, len(events))
	for i, e := range events {
		ret[i] = EventToEvent(e.Event)
	}
	return ret
}

func EventToEvent(in *cloudevents.Event) conformanceevent.Event {
	if in == nil {
		return conformanceevent.Event{}
	}
	out := conformanceevent.Event{
		Attributes: conformanceevent.ContextAttributes{
			SpecVersion:     in.SpecVersion(),
			Type:            in.Type(),
			ID:              in.ID(),
			Source:          in.Source(),
			Subject:         in.Subject(),
			DataSchema:      in.DataSchema(),
			DataContentType: in.DataContentType(),
			Extensions:      make(conformanceevent.Extensions),
		},
		Data: string(in.Data()),
	}
	if !in.Time().IsZero() {
		out.Attributes.Time = types.Timestamp{Time: in.Time()}.String()
	}
	for key, ext := range in.Extensions() {
		if value, err := types.Format(ext); err == nil {
			out.Attributes.Extensions[key] = value
		}
	}

	return out
}

func PatternFromProber(ctx context.Context, prober *eventshub.EventProber, prefix string) EventPattern {
	events := prober.ReceivedOrRejectedBy(ctx, prefix)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	got := EventPattern{
		Success:  make([]bool, 0),
		Interval: make([]uint, 0),
	}
	for i, event := range events {
		got.Success = append(got.Success, event.Kind == eventshub.EventReceived)
		if i == 0 {
			got.Interval = []uint{0}
		} else {
			diff := events[i-1].Time.Unix() - event.Time.Unix()
			got.Interval = append(got.Interval, uint(diff))
		}
	}
	return got
}

func PatternFromEstimates(attempts, failures uint) EventPattern {
	p := EventPattern{
		Success:  []bool{},
		Interval: []uint{},
	}
	for i := uint(0); i < attempts; i++ {
		if i >= failures {
			p.Success = append(p.Success, true)
			p.Interval = append(p.Interval, 0) // TODO: calculate time.
			return p
		}
		p.Success = append(p.Success, false)
		p.Interval = append(p.Interval, 0) // TODO: calculate time.
	}
	return p
}

func AssertEventPatterns(prober *eventshub.EventProber, expected map[string]EventPattern) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for prefix, want := range expected {
			got := PatternFromProber(ctx, prober, prefix)

			t.Logf("Expected Events %s; \n Got: %#v\nWant: %#v", prefix, got, want)

			if len(want.Success) != len(got.Success) {
				t.Errorf("Wanted %d events, got %d", len(want.Success), len(got.Success))
			}

			// Check event acceptance.
			if len(want.Success) != 0 && len(got.Success) != 0 {
				if diff := cmp.Diff(want.Success, got.Success); diff != "" {
					t.Error("unexpected event acceptance behaviour (-want, +got) =", diff)
				}
			}
			// Check timing.
			//if len(want.eventInterval) != 0 && len(got.eventInterval) != 0 {
			//	if diff := cmp.Diff(want.eventInterval, got.eventInterval); diff != "" {
			//		t.Error("unexpected event Interval behaviour (-want, +got) =", diff)
			//	}
			//}
		}
	}
}

// DeliveryAttempts will return the number of times the first non-nil delivery
// spec declares retry to be. The returned value will be 1 + retry.
func DeliveryAttempts(specs ...*v1.DeliverySpec) uint {
	for _, spec := range specs {
		if spec != nil {
			if spec.Retry == nil || *spec.Retry == 0 {
				return 1
			}
			return 1 + uint(*spec.Retry)
		}
	}

	return 1
}
