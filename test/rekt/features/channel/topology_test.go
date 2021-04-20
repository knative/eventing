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

package channel

import (
	"reflect"
	"testing"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

func Test_createExpectedEventPattern(t *testing.T) {
	type args struct {
		chDS *v1.DeliverySpec
		subs []subCfg
	}
	tests := []struct {
		name string
		args args
		want map[string]eventPattern
	}{{
		name: "no sub",
		args: args{
			chDS: nil,
			subs: nil,
		},
		want: map[string]eventPattern{
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   0,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1reply": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, no reply",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     false,
				subFailCount:   0,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1reply": {
				success:  []bool{},
				interval: []uint{},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, fails subscriber, no delivery",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     false,
				subFailCount:   1,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false},
				interval: []uint{0},
			},
			"sub1reply": {
				success:  []bool{},
				interval: []uint{},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, fails reply, no delivery",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   0,
				hasReply:       true,
				replyFailCount: 1,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1reply": {
				success:  []bool{false},
				interval: []uint{0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, fails 3x, ch delivery",
		args: args{
			chDS: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(3),
			},
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   3,
				hasReply:       true,
				replyFailCount: 3,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, fails 4x, ch delivery",
		args: args{
			chDS: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(3),
			},
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   4,
				hasReply:       true,
				replyFailCount: 4,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, false},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{},
				interval: []uint{},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{true},
				interval: []uint{0},
			},
		},
	}, {
		name: "one sub, fails 3/4x, ch delivery",
		args: args{
			chDS: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(3),
			},
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   3,
				hasReply:       true,
				replyFailCount: 4,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{false, false, false, false},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{true},
				interval: []uint{0},
			},
		},
	}, {
		name: "one sub, fails 3/4x, sub delivery",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   3,
				hasReply:       true,
				replyFailCount: 4,
				delivery: &v1.DeliverySpec{
					DeadLetterSink: new(duckv1.Destination),
					Retry:          ptr.Int32(3),
				},
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{false, false, false, false},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, fails 3/4x, ch delivery",
		args: args{
			chDS: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(3),
			},
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   3,
				hasReply:       true,
				replyFailCount: 4,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{false, false, false, false},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{true},
				interval: []uint{0},
			},
		},
	}, {
		name: "one sub, fails 3/4x, both delivery",
		args: args{
			chDS: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(10),
			},
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   3,
				hasReply:       true,
				replyFailCount: 4,
				delivery: &v1.DeliverySpec{
					DeadLetterSink: new(duckv1.Destination),
					Retry:          ptr.Int32(3),
				},
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{false, false, false, true},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				success:  []bool{false, false, false, false},
				interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "one sub, no subscriber",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         false,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{},
				interval: []uint{},
			},
			"sub1reply": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}, {
		name: "two subs",
		args: args{
			chDS: nil,
			subs: []subCfg{{
				prefix:         "sub1",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   0,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}, {
				prefix:         "sub2",
				hasSub:         true,
				subReplies:     true,
				subFailCount:   0,
				hasReply:       true,
				replyFailCount: 0,
				delivery:       nil,
			}},
		},
		want: map[string]eventPattern{
			"sub1sub": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1reply": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub1dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"sub2sub": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub2reply": {
				success:  []bool{true},
				interval: []uint{0},
			},
			"sub2dlq": {
				success:  []bool{},
				interval: []uint{},
			},
			"chdlq": {
				success:  []bool{},
				interval: []uint{},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createExpectedEventPatterns(tt.args.chDS, tt.args.subs)
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("%s: Maps unequal: want:\n%+v\ngot:\n%+v", tt.name, tt.want, got)
			}
		})
	}
}

func Test_deliveryAttempts(t *testing.T) {
	type args struct {
		p0 *v1.DeliverySpec
		p1 *v1.DeliverySpec
	}
	tests := []struct {
		name string
		args args
		want uint
	}{{
		name: "none",
		args: args{
			p0: nil,
			p1: nil,
		},
		want: 1,
	}, {
		name: "just p0",
		args: args{
			p0: &v1.DeliverySpec{
				Retry: ptr.Int32(3),
			},
			p1: nil,
		},
		want: 4,
	}, {
		name: "just p1",
		args: args{
			p0: nil,
			p1: &v1.DeliverySpec{
				Retry: ptr.Int32(3),
			},
		},
		want: 4,
	}, {
		name: "both",
		args: args{
			p0: &v1.DeliverySpec{
				Retry: ptr.Int32(2),
			},
			p1: &v1.DeliverySpec{
				Retry: ptr.Int32(3),
			},
		},
		want: 3,
	}, {
		name: "just p0, no retry",
		args: args{
			p0: &v1.DeliverySpec{},
			p1: nil,
		},
		want: 1,
	}, {
		name: "just p1, no retry",
		args: args{
			p0: nil,
			p1: &v1.DeliverySpec{},
		},
		want: 1,
	}, {
		name: "both, p1 retry",
		args: args{
			p0: &v1.DeliverySpec{},
			p1: &v1.DeliverySpec{
				Retry: ptr.Int32(3),
			},
		},
		want: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deliveryAttempts(tt.args.p0, tt.args.p1); got != tt.want {
				t.Errorf("deliveryAttempts() = %v, want %v", got, tt.want)
			}
		})
	}
}
