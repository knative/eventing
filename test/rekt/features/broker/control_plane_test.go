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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

var noEvents = expectedEvents{
	eventSuccess:  []bool{},
	eventInterval: []uint{},
}

var oneSuccessfulEvent = expectedEvents{
	eventSuccess:  []bool{true},
	eventInterval: []uint{0},
}

var dlqSink = &pkgduckv1.Destination{
	Ref: &pkgduckv1.KReference{
		Kind:      "doesnotmatter",
		Namespace: "yah",
		Name:      "sumtin",
	},
}

func TestHelper(t *testing.T) {
	for _, tt := range []struct {
		retry    uint
		failures uint
		want     expectedEvents
	}{{
		retry:    0,
		failures: 1,
		want: expectedEvents{
			eventSuccess:  []bool{false},
			eventInterval: []uint{0},
		},
	}, {
		retry:    0,
		failures: 0,
		want: expectedEvents{
			eventSuccess:  []bool{true},
			eventInterval: []uint{0},
		},
	}, {
		retry:    0,
		failures: 5,
		want: expectedEvents{
			eventSuccess:  []bool{false},
			eventInterval: []uint{0},
		},
	}, {
		retry:    3,
		failures: 0,
		want: expectedEvents{
			eventSuccess:  []bool{true},
			eventInterval: []uint{0},
		},
	}, {
		retry:    3,
		failures: 2,
		want: expectedEvents{
			eventSuccess:  []bool{false, false, true},
			eventInterval: []uint{0, 0, 0},
		},
	}, {
		// Test with 3 retries => 4 sends altogether(one initial, plus 3 retries)
		retry:    3,
		failures: 4,
		want: expectedEvents{
			eventSuccess:  []bool{false, false, false, false},
			eventInterval: []uint{0, 0, 0, 0},
		},
	}} {
		got := helper(tt.retry, tt.failures, false)
		if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(expectedEvents{})); diff != "" {
			t.Log("Unexpected out: ", diff)
		}
	}
}

func TestCreateExpectedEventMap(t *testing.T) {
	//	two := int32(2)

	for _, tt := range []struct {
		name        string
		brokerDS    *v1.DeliverySpec
		t1DS        *v1.DeliverySpec
		t2DS        *v1.DeliverySpec
		t1FailCount uint
		t2FailCount uint
		want        map[string]expectedEvents
	}{{
		name:        "no retries, no failures, both t1 / t2 get it, nothing else",
		brokerDS:    nil,
		t1DS:        nil,
		t2DS:        nil,
		t1FailCount: 0,
		t2FailCount: 0,
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{false, false},
				eventInterval: []uint{0, 0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{false, false, false, true},
				eventInterval: []uint{0, 0, 0, 0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{false, false, false, false},
				eventInterval: []uint{0, 0, 0, 0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			},
			"t2": {
				eventSuccess:  []bool{false, true},
				eventInterval: []uint{0, 0},
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
		want: map[string]expectedEvents{
			"t1": {
				eventSuccess:  []bool{false, true},
				eventInterval: []uint{0, 0},
			},
			"t2": {
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": oneSuccessfulEvent,
		},
	}} {
		got := createExpectedEventMap(tt.brokerDS, tt.t1DS, tt.t2DS, tt.t1FailCount, tt.t2FailCount)
		if !reflect.DeepEqual(tt.want, got) {
			t.Logf("%s: Maps unequal: want:\n%+v\ngot:\n%+v", tt.name, tt.want, got)
		}
	}
}
