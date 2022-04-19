/*
Copyright 2020 The Knative Authors

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

package v1beta1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	pkgduck "knative.dev/pkg/apis/duck/v1"
)

func TestDeliverySpecConversionBadType(t *testing.T) {
	good, bad := &DeliverySpec{}, &DeliverySpec{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1beta1 -> v1 -> v1beta1
func TestDeliverySpecConversion(t *testing.T) {
	var retryCount int32 = 10
	var backoffPolicy BackoffPolicyType = BackoffPolicyLinear
	var backoffPolicyExp BackoffPolicyType = BackoffPolicyExponential
	var backoffPolicyBad BackoffPolicyType = "garbage"
	badPolicyString := `unknown BackoffPolicy, got: "garbage"`

	tests := []struct {
		name string
		in   *DeliverySpec
		err  *string
	}{{
		name: "min configuration",
		in: &DeliverySpec{
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with retry",
		in: &DeliverySpec{
			Retry: &retryCount,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with linear backoff",
		in: &DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicy,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with exp backoff",
		in: &DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicyExp,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with bad backoff",
		in: &DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicyBad,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
		err: &badPolicyString,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ver := &v1.DeliverySpec{}
			err := test.in.ConvertTo(context.Background(), ver)
			if err != nil {
				if test.err == nil || *test.err != err.Error() {
					t.Error("ConvertTo() =", err)
				}
				return
			}
			got := &DeliverySpec{}
			if err := got.ConvertFrom(context.Background(), ver); err != nil {
				t.Error("ConvertFrom() =", err)
			}
			if diff := cmp.Diff(test.in, got); diff != "" {
				t.Error("roundtrip (-want, +got) =", diff)
			}
		})
	}
}

// Test v1 -> v1beta1 -> v1
func TestDeliverySpecConversionV1(t *testing.T) {
	var retryCount int32 = 10
	var backoffPolicy v1.BackoffPolicyType = v1.BackoffPolicyLinear
	var backoffPolicyExp v1.BackoffPolicyType = v1.BackoffPolicyExponential
	var backoffPolicyBad v1.BackoffPolicyType = "garbage"
	badPolicyString := `unknown BackoffPolicy, got: "garbage"`

	tests := []struct {
		name string
		in   *v1.DeliverySpec
		err  *string
	}{{
		name: "min configuration",
		in: &v1.DeliverySpec{
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with retry",
		in: &v1.DeliverySpec{
			Retry: &retryCount,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with linear backoff",
		in: &v1.DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicy,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with exp backoff",
		in: &v1.DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicyExp,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}, {
		name: "with bad backoff",
		in: &v1.DeliverySpec{
			Retry:         &retryCount,
			BackoffPolicy: &backoffPolicyBad,
			DeadLetterSink: &pkgduck.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
		err: &badPolicyString,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ver := &DeliverySpec{}
			err := ver.ConvertFrom(context.Background(), test.in)
			if err != nil {
				if test.err == nil || *test.err != err.Error() {
					t.Error("ConvertFrom() =", err)
				}
				return
			}
			got := &v1.DeliverySpec{}
			if err := ver.ConvertTo(context.Background(), got); err != nil {
				t.Error("ConvertTo() =", err)
			}
			if diff := cmp.Diff(test.in, got); diff != "" {
				t.Error("roundtrip (-want, +got) =", diff)
			}
		})
	}
}

// v1beta1 and v1 DeliveryStatus are not convertable to each other.
// (channel ref vs apis.URL)
