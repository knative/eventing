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

	// RetryAfterMax provides an optional upper bound on the duration specified in a "Retry-After" header
	// when calculating backoff times for retrying 429 and 503 response codes.  Setting the value to
	// zero ("PT0S") can be used to opt-out of respecting "Retry-After" header values altogether. This
	// value only takes effect if "Retry" is configured, and also depends on specific implementations
	// (Channels, Sources, etc.) choosing to provide this capability.
	//
	// Note: This API is EXPERIMENTAL and might be changed at anytime. While this experimental
	//       feature is in the Alpha/Beta stage, you must provide a valid value to opt-in for
	//       supporting "Retry-After" headers.  When the feature becomes Stable/GA "Retry-After"
	//       headers will be respected by default, and you can choose to specify "PT0S" to
	//       opt-out of supporting "Retry-After" headers.
	//       For more details: https://github.com/knative/eventing/issues/5811
	//
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	//
	// +optional
	RetryAfterMax *string `json:"retryAfterMax,omitempty"`
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

	if ds.RetryAfterMax != nil {
		if feature.FromContext(ctx).IsEnabled(feature.DeliveryRetryAfter) {
			p, me := period.Parse(*ds.RetryAfterMax)
			if me != nil || p.IsNegative() {
				errs = errs.Also(apis.ErrInvalidValue(*ds.RetryAfterMax, "retryAfterMax"))
			}
		} else {
			errs = errs.Also(apis.ErrDisallowedFields("retryAfterMax"))
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
	// DeadLetterSinkCACerts are Certification Authority (CA) certificates in PEM format
	// according to https://www.rfc-editor.org/rfc/rfc7468.
	// +optional
	DeadLetterSinkCACerts *string `json:"deadLetterSinkCACerts,omitempty"`
	// DeadLetterSinkAudience is the OIDC audience of the DeadLetterSink
	// +optional
	DeadLetterSinkAudience *string `json:"deadLetterSinkAudience,omitempty"`
}

func (ds *DeliveryStatus) IsSet() bool {
	return ds.DeadLetterSinkURI != nil
}

func NewDeliveryStatusFromAddressable(addr *duckv1.Addressable) DeliveryStatus {
	return DeliveryStatus{
		DeadLetterSinkURI:      addr.URL,
		DeadLetterSinkCACerts:  addr.CACerts,
		DeadLetterSinkAudience: addr.Audience,
	}
}

func NewDestinationFromDeliveryStatus(status DeliveryStatus) duckv1.Destination {
	return duckv1.Destination{
		URI:      status.DeadLetterSinkURI,
		CACerts:  status.DeadLetterSinkCACerts,
		Audience: status.DeadLetterSinkAudience,
	}
}
