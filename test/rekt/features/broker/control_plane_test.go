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
)

var noEvents = expectedEvents{
	eventSuccess:  []bool{},
	eventInterval: []int{},
}

var oneSuccessfulEvent = expectedEvents{
	eventSuccess:  []bool{true},
	eventInterval: []int{0},
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
		retry    int
		failures int
		want     expectedEvents
	}{{
		retry:    0,
		failures: 1,
		want: expectedEvents{
			eventSuccess:  []bool{false},
			eventInterval: []int{0},
		},
	}, {
		retry:    0,
		failures: 0,
		want: expectedEvents{
			eventSuccess:  []bool{true},
			eventInterval: []int{0},
		},
	}, {
		retry:    0,
		failures: 5,
		want: expectedEvents{
			eventSuccess:  []bool{false},
			eventInterval: []int{0},
		},
	}, {
		retry:    3,
		failures: 0,
		want: expectedEvents{
			eventSuccess:  []bool{true},
			eventInterval: []int{0},
		},
	}, {
		retry:    3,
		failures: 2,
		want: expectedEvents{
			eventSuccess:  []bool{false, false, true},
			eventInterval: []int{0, 0, 0},
		},
	}} {
		got := helper(tt.retry, tt.failures, false)
		if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(expectedEvents{})); diff != "" {
			t.Log("Unexpected out: ", diff)
		}
	}
}

func TestCreateExpectedEventMap(t *testing.T) {
	one := int32(1)
	//	two := int32(2)

	for _, tt := range []struct {
		name        string
		brokerDS    *v1.DeliverySpec
		t1DS        *v1.DeliverySpec
		t2DS        *v1.DeliverySpec
		t1FailCount int
		t2FailCount int
		want        map[string]expectedEvents
	}{{
		name:        "no retries, no failures, both t1 / t2 get it, nothing else",
		brokerDS:    nil,
		t1DS:        nil,
		t2DS:        nil,
		t1FailCount: 0,
		t2FailCount: 0,
		want: map[string]expectedEvents{
			"t1": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
			},
			"t2": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, one retry, no failures, both t1 / t2 get it, nothing else",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry: &one,
		},
		t2DS:        nil,
		t1FailCount: 0,
		t2FailCount: 0,
		want: map[string]expectedEvents{
			"t1": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
			},
			"t2": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     noEvents,
			"brokerdlq": noEvents,
		},
	}, {
		name:     "t1, one retry, one failure, both t1 / t2 get it, t1dlq gets it too",
		brokerDS: nil,
		t1DS: &v1.DeliverySpec{
			Retry:          &one,
			DeadLetterSink: dlqSink,
		},
		t2DS:        nil,
		t1FailCount: 1,
		t2FailCount: 0,
		want: map[string]expectedEvents{
			"t1": expectedEvents{
				eventSuccess:  []bool{false},
				eventInterval: []int{0},
			},
			"t2": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
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
			Retry:          &one,
			DeadLetterSink: dlqSink,
		},
		t1FailCount: 0,
		t2FailCount: 1,
		want: map[string]expectedEvents{
			"t1": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
			},
			"t2": expectedEvents{
				eventSuccess:  []bool{false},
				eventInterval: []int{0},
			},
			"t1dlq":     noEvents,
			"t2dlq":     oneSuccessfulEvent,
			"brokerdlq": noEvents,
		},
	}, {
		name: "t1, one retry, one failure, both t1 / t2 get it, brokerdlq gets it too",
		brokerDS: &v1.DeliverySpec{
			Retry:          &one,
			DeadLetterSink: dlqSink,
		},
		t1DS:        nil,
		t2DS:        nil,
		t1FailCount: 1,
		t2FailCount: 0,
		want: map[string]expectedEvents{
			"t1": expectedEvents{
				eventSuccess:  []bool{false},
				eventInterval: []int{0},
			},
			"t2": expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []int{0},
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

		//		if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(map[string]expectedEvents{})); diff != "" {
		//			t.Log("Unexpected diff: ", diff)
		//		}
	}
}
