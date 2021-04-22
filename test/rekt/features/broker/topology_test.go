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

package broker

import (
	"knative.dev/eventing/test/rekt/features/knconf"
	"reflect"
	"testing"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

var noEvents = knconf.EventPattern{
	Success:  []bool{},
	Interval: []uint{},
}

var oneSuccessfulEvent = knconf.EventPattern{
	Success:  []bool{true},
	Interval: []uint{0},
}

var dlqSink = &pkgduckv1.Destination{
	Ref: &pkgduckv1.KReference{
		Kind:      "doesnotmatter",
		Namespace: "yah",
		Name:      "sumtin",
	},
}

func TestCreateExpectedEventPatterns(t *testing.T) {
	for _, tt := range []struct {
		name        string
		brokerDS    *v1.DeliverySpec
		t1DS        *v1.DeliverySpec
		t2DS        *v1.DeliverySpec
		t1FailCount uint
		t2FailCount uint
		want        map[string]knconf.EventPattern
	}{{
		name:        "no retries, no failures, both t1 / t2 get it, nothing else",
		brokerDS:    nil,
		t1DS:        nil,
		t2DS:        nil,
		t1FailCount: 0,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, one retry, no failures, both t1 / t2 get it, nothing else",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry: ptr.Int32(1),
		},
		t2DS:        nil,
		t1FailCount: 0,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, one retry, one failure, both t1 / t2 get it, t1dlq gets it too",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry:          ptr.Int32(1),
			DeadLetterSink: dlqSink,
		},
		t2DS:        nil,
		t1FailCount: 2,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{false, false},
				Interval: []uint{0, 0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     oneSuccessfulEvent,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, four retries with linear, 5 seconds apart, 3 failures, both t1 / t2 get it, no dlq gets it because succeeds on 4th try",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry:          ptr.Int32(4),
			DeadLetterSink: dlqSink,
		},
		t2DS:        nil,
		t1FailCount: 3,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, three retries with linear, 5 seconds apart, 4 failures, both t1 / t2 get it, t1dlq gets it because fails all 4 tries",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry:          ptr.Int32(3),
			DeadLetterSink: dlqSink,
		},
		t2DS:        nil,
		t1FailCount: 4,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     oneSuccessfulEvent,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t2, one retry, one failure, both t1 / t2 get it, t2dlq gets it too",
		brokerDS: nil,
		t1DS:     nil,
		t2DS: &v1.DeliverySpec{
			Retry:          ptr.Int32(1),
			DeadLetterSink: dlqSink,
		},
		t1FailCount: 0,
		t2FailCount: 1,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t2": {
				Success:  []bool{false, true},
				Interval: []uint{0, 0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     oneSuccessfulEvent,
			"brokerdlq": noEvents,
		},
	}, {
		name: "t1, one retry, one failure, both t1 / t2 get it, brokerdlq gets it too",
		brokerDS: &v1.DeliverySpec{
			Retry:          ptr.Int32(1),
			DeadLetterSink: dlqSink,
		},
		t1DS:        nil,
		t2DS:        nil,
		t1FailCount: 1,
		t2FailCount: 0,
		want: map[string]knconf.EventPattern{
			"t1": {
				Success:  []bool{false, true},
				Interval: []uint{0, 0},
			},
			"t2": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": oneSuccessfulEvent,
		},
	}} {
		cfg := []triggerCfg{{
			delivery:  tt.t1DS,
			failCount: tt.t1FailCount,
		}, {
			delivery:  tt.t2DS,
			failCount: tt.t2FailCount,
		}}
		got := createExpectedEventPatterns(tt.brokerDS, cfg)
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("%s: Maps unequal: want:\n%+v\ngot:\n%+v", tt.name, tt.want, got)
		}
	}
}

func TestEventMatchesTrigger(t *testing.T) {
	for _, tt := range []struct {
		name   string
		event  conformanceevent.Event
		filter *eventingv1.TriggerFilter
		want   bool
	}{{
		name:   "nil filter matches everything",
		event:  conformanceevent.Event{},
		filter: nil,
		want:   true,
	}, {
		name: "correct match on type",
		event: conformanceevent.Event{
			Attributes: conformanceevent.ContextAttributes{
				Type: "eventtype",
			},
		},
		filter: &eventingv1.TriggerFilter{
			Attributes: eventingv1.TriggerFilterAttributes{
				"type": "eventtype",
			},
		},
		want: true,
	}, {
		name: "wrong type",
		event: conformanceevent.Event{
			Attributes: conformanceevent.ContextAttributes{
				Type: "eventtype",
			},
		},
		filter: &eventingv1.TriggerFilter{
			Attributes: eventingv1.TriggerFilterAttributes{
				"type": "wrongtype",
			},
		},
		want: false,
	}} {
		if got := eventMatchesTrigger(tt.event, tt.filter); got != tt.want {
			t.Errorf("%q : Event: %+v Trigger: %+v wanted match: %t but got %t", tt.name, tt.event, tt.filter, tt.want, got)
		}
	}
}

func TestCreateExpectedEventDeliveryMap(t *testing.T) {
	for _, tt := range []struct {
		name     string
		inevents []conformanceevent.Event
		config   []triggerCfg
		want     map[string][]conformanceevent.Event
	}{{
		name: "nil filter matches everything",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: make([]triggerCfg, 1), // One trigger, no filter, no reply
		want: map[string][]conformanceevent.Event{
			"t0": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "eventtype",
					},
				},
			},
		},
	}, {
		name: "wrong event type, no match ",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "wrongtype",
					},
				},
			},
		},
		want: map[string][]conformanceevent.Event{},
	}, {
		name: "Two triggers, incoming matches the second one (t1), not first (t0). Reply from t1 matches t0",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "replyeventtype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "eventtype",
					},
				},
				reply: &conformanceevent.Event{
					Attributes: conformanceevent.ContextAttributes{
						Type: "replyeventtype",
					},
				},
			},
		},
		want: map[string][]conformanceevent.Event{
			"t0": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "replyeventtype",
					},
				},
			},
			"t1": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "eventtype",
					},
				},
			},
		},
	}, {
		name: "Two triggers, incoming matches no triggers, hence there's no reply. Reply from t0 matches nothing",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "replyeventtype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "eventtypenomatch",
					},
				},
				reply: &conformanceevent.Event{
					Attributes: conformanceevent.ContextAttributes{
						Type: "replyeventtype",
					},
				},
			},
		},
		want: map[string][]conformanceevent.Event{},
	}, {
		name: "Two triggers, matches the second one (t1), not first (t0)",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "wrongtype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "eventtype",
					},
				},
			},
		},
		want: map[string][]conformanceevent.Event{
			"t1": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "eventtype",
					},
				},
			},
		},
	}, {
		name: "Two triggers, both get the same event",
		inevents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "eventtype",
				},
			},
		},
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "eventtype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "eventtype",
					},
				},
			},
		},
		want: map[string][]conformanceevent.Event{
			"t0": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "eventtype",
					},
				},
			},
			"t1": {
				{
					Attributes: conformanceevent.ContextAttributes{
						Type: "eventtype",
					},
				},
			},
		},
	}} {
		got := createExpectedEventRoutingMap(tt.config, tt.inevents)
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("%s: Maps unequal: want:\n%+v\ngot:\n%+v", tt.name, tt.want, got)
		}
	}
}
