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

package v1

import (
	"context"

	"github.com/rickb777/date/period"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/feature"
)

// DeliverySpec contains the delivery options for event senders,
// such as channelable and source.
type DeliverySpec struct {
	// DeadLetterSink is the sink receiving event that could not be sent to
	// a destination.
	// +optional
	DeadLetterSink *duckv1.Destination `json:"deadLetterSink,omitempty"`

	// Retry is the minimum number of retries the sender should attempt when
	// sending an event before moving it to the dead letter sink.
	// +optional
	Retry *int32 `json:"retry,omitempty"`

	// Timeout is the timeout of each single request. The value must be greater than 0.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	//
	// Note: This API is EXPERIMENTAL and might break anytime. For more details: https://github.com/knative/eventing/issues/5148
	// +optional
	Timeout *string `json:"timeout,omitempty"`

	// BackoffPolicy is the retry backoff policy (linear, exponential).
	// +optional
	BackoffPolicy *BackoffPolicyType `json:"backoffPolicy,omitempty"`

	// BackoffDelay is the delay before retrying.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	//
	// For linear policy, backoff delay is backoffDelay*<numberOfRetries>.
	// For exponential policy, backoff delay is backoffDelay*2^<numberOfRetries>.
	// +optional
	BackoffDelay *string `json:"backoffDelay,omitempty"`

	// RetryAfter controls how "Retry-After" header durations are handled for 429 and 503 response codes.
	// If not provided, the default behavior is to ignore "Retry-After" headers in responses.
	//
	// Note: This API is EXPERIMENTAL and might break anytime. For more details: https://github.com/knative/eventing/issues/5811
	// +optional
	RetryAfter *RetryAfter `json:"retryAfter,omitempty"`
}

// RetryAfter contains configuration related to the handling of "Retry-After" headers.
type RetryAfter struct {
	// Enabled is a flag indicating whether to respect the "Retry-After" header duration.
	// If enabled, the largest of the normal backoff duration and the "Retry-After"
	// header value will be used when calculating the next backoff duration.  This will
	// only be considered when a 429 (Too Many Requests) or 503 (Service Unavailable)
	// response code is received and Retry is greater than 0.
	Enabled bool `json:"enabled"`

	// MaxDuration is the maximum time to wait before retrying.  It is intended as an
	// override to protect against excessively large "Retry-After" durations. If provided,
	// the value must be greater than 0.  If not provided, the largest of the "Retry-After"
	// duration and the normal backoff duration will be used.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	// +optional
	MaxDuration *string `json:"maxDuration,omitempty"`
}

func (ds *DeliverySpec) Validate(ctx context.Context) *apis.FieldError {
	if ds == nil {
		return nil
	}
	var errs *apis.FieldError
	if dlse := ds.DeadLetterSink.Validate(ctx); dlse != nil {
		errs = errs.Also(dlse).ViaField("deadLetterSink")
	}

	if ds.Retry != nil && *ds.Retry < 0 {
		errs = errs.Also(apis.ErrInvalidValue(*ds.Retry, "retry"))
	}

	if ds.Timeout != nil {
		if feature.FromContext(ctx).IsEnabled(feature.DeliveryTimeout) {
			t, te := period.Parse(*ds.Timeout)
			if te != nil || t.IsZero() {
				errs = errs.Also(apis.ErrInvalidValue(*ds.Timeout, "timeout"))
			}
		} else {
			errs = errs.Also(apis.ErrDisallowedFields("timeout"))
		}
	}

	if ds.BackoffPolicy != nil {
		switch *ds.BackoffPolicy {
		case BackoffPolicyExponential, BackoffPolicyLinear:
			// nothing
		default:
			errs = errs.Also(apis.ErrInvalidValue(*ds.BackoffPolicy, "backoffPolicy"))
		}
	}

	if ds.BackoffDelay != nil {
		_, te := period.Parse(*ds.BackoffDelay)
		if te != nil {
			errs = errs.Also(apis.ErrInvalidValue(*ds.BackoffDelay, "backoffDelay"))
		}
	}

	if ds.RetryAfter != nil {
		if feature.FromContext(ctx).IsEnabled(feature.DeliveryRetryAfter) {
			if ds.RetryAfter.MaxDuration != nil && *ds.RetryAfter.MaxDuration != "" {
				m, me := period.Parse(*ds.RetryAfter.MaxDuration)
				if me != nil || m.IsZero() {
					errs = errs.Also(apis.ErrInvalidValue(*ds.RetryAfter.MaxDuration, "retryAfter.maxDuration"))
				}
			}
		} else {
			errs = errs.Also(apis.ErrDisallowedFields("retryAfter"))
		}
	}

	return errs
}

// BackoffPolicyType is the type for backoff policies
type BackoffPolicyType string

const (
	// Linear backoff policy
	BackoffPolicyLinear BackoffPolicyType = "linear"

	// Exponential backoff policy
	BackoffPolicyExponential BackoffPolicyType = "exponential"
)

// DeliveryStatus contains the Status of an object supporting delivery options. This type is intended to be embedded into a status struct.
type DeliveryStatus struct {
	// DeadLetterSink is a KReference that is the reference to the native, platform specific channel
	// where failed events are sent to.
	// +optional
	DeadLetterSinkURI *apis.URL `json:"deadLetterSinkUri,omitempty"`
}
