package knconf

import (
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"testing"
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
