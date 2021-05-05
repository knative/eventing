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
	"knative.dev/eventing/test/rekt/features/knconf"
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
		want map[string]knconf.EventPattern
	}{{
		name: "no sub",
		args: args{
			chDS: nil,
			subs: nil,
		},
		want: map[string]knconf.EventPattern{
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1reply": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1reply": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false},
				Interval: []uint{0},
			},
			"sub1reply": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1reply": {
				Success:  []bool{false},
				Interval: []uint{0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{true},
				Interval: []uint{0},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{true},
				Interval: []uint{0},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{true},
				Interval: []uint{0},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{false, false, false, true},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1reply": {
				Success:  []bool{false, false, false, false},
				Interval: []uint{0, 0, 0, 0},
			},
			"sub1dlq": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"sub1reply": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
		want: map[string]knconf.EventPattern{
			"sub1sub": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1reply": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub1dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"sub2sub": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub2reply": {
				Success:  []bool{true},
				Interval: []uint{0},
			},
			"sub2dlq": {
				Success:  []bool{},
				Interval: []uint{},
			},
			"chdlq": {
				Success:  []bool{},
				Interval: []uint{},
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
