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
	"testing"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

func TestDeliveryAttempts(t *testing.T) {
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
			if got := DeliveryAttempts(tt.args.p0, tt.args.p1); got != tt.want {
				t.Errorf("deliveryAttempts() = %v, want %v", got, tt.want)
			}
		})
	}
}
