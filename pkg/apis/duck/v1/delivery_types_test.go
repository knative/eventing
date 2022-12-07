/*
Copyright 2020 The Knative Authors. All Rights Reserved.
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

package v1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/feature"
)

func TestDeliverySpecValidation(t *testing.T) {
	deliveryTimeoutEnabledCtx := feature.ToContext(context.TODO(), feature.Flags{
		feature.DeliveryTimeout: feature.Enabled,
	})
	deliveryRetryAfterEnabledCtx := feature.ToContext(context.TODO(), feature.Flags{
		feature.DeliveryRetryAfter: feature.Enabled,
	})

	invalidString := "invalid time"
	bop := BackoffPolicyExponential
	validDuration := "PT2S"
	invalidDuration := "1985-04-12T23:20:50.52Z"
	tests := []struct {
		name string
		spec *DeliverySpec
		ctx  context.Context
		want *apis.FieldError
	}{{
		name: "nil is valid",
		spec: nil,
		want: nil,
	}, {
		name: "invalid time format",
		spec: &DeliverySpec{BackoffDelay: &invalidString},
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue(invalidString, "backoffDelay")
		}(),
	}, {
		name: "invalid deadLetterSink",
		spec: &DeliverySpec{DeadLetterSink: &duckv1.Destination{}},
		want: func() *apis.FieldError {
			return apis.ErrGeneric("expected at least one, got none", "ref", "uri").ViaField("deadLetterSink")
		}(),
	}, {
		name: "valid timeout",
		spec: &DeliverySpec{Timeout: &validDuration},
		ctx:  deliveryTimeoutEnabledCtx,
		want: nil,
	}, {
		name: "invalid timeout",
		spec: &DeliverySpec{Timeout: &invalidDuration},
		ctx:  deliveryTimeoutEnabledCtx,
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue(invalidDuration, "timeout")
		}(),
	}, {
		name: "zero timeout",
		spec: &DeliverySpec{Timeout: pointer.String("PT0S")},
		ctx:  deliveryTimeoutEnabledCtx,
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue("PT0S", "timeout")
		}(),
	}, {
		name: "disabled timeout",
		spec: &DeliverySpec{Timeout: &validDuration},
		want: apis.ErrDisallowedFields("timeout"),
	}, {
		name: "valid backoffPolicy",
		spec: &DeliverySpec{BackoffPolicy: &bop},
		want: nil,
	}, {
		name: "valid backoffDelay",
		spec: &DeliverySpec{BackoffDelay: &validDuration},
		want: nil,
	}, {
		name: "invalid backoffDelay",
		spec: &DeliverySpec{BackoffDelay: &invalidDuration},
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue(invalidDuration, "backoffDelay")
		}(),
	}, {
		name: "negative retry",
		spec: &DeliverySpec{Retry: pointer.Int32(-1)},
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue("-1", "retry")
		}(),
	}, {
		name: "valid retry 0",
		spec: &DeliverySpec{Retry: pointer.Int32(0)},
		want: nil,
	}, {
		name: "valid retry 1",
		spec: &DeliverySpec{Retry: pointer.Int32(1)},
		want: nil,
	}, {
		name: "valid retryAfterMax",
		ctx:  deliveryRetryAfterEnabledCtx,
		spec: &DeliverySpec{RetryAfterMax: &validDuration},
		want: nil,
	}, {
		name: "zero retryAfterMax",
		ctx:  deliveryRetryAfterEnabledCtx,
		spec: &DeliverySpec{RetryAfterMax: pointer.String("PT0S")},
		want: nil,
	}, {
		name: "empty retryAfterMax",
		ctx:  deliveryRetryAfterEnabledCtx,
		spec: &DeliverySpec{RetryAfterMax: pointer.String("")},
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue("", "retryAfterMax")
		}(),
	}, {
		name: "invalid retryAfterMax",
		ctx:  deliveryRetryAfterEnabledCtx,
		spec: &DeliverySpec{RetryAfterMax: &invalidDuration},
		want: func() *apis.FieldError {
			return apis.ErrInvalidValue(invalidDuration, "retryAfterMax")
		}(),
	}, {
		name: "disabled feature with retryAfterMax",
		spec: &DeliverySpec{RetryAfterMax: &validDuration},
		want: func() *apis.FieldError {
			return apis.ErrDisallowedFields("retryAfterMax")
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := test.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			got := test.spec.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("DeliverySpec.Validate (-want, +got) =", diff)
			}
		})
	}
}
