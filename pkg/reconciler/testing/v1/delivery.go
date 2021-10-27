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

package testing

import (
	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
)

// DeliveryOption enables configuration of Delivery options.
type DeliveryOption func(*eventingv1.DeliverySpec)

// NewDelivery creates a Delivery with DeliveryOptions
func NewDelivery(opts ...DeliveryOption) *eventingv1.DeliverySpec {
	d := &eventingv1.DeliverySpec{}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// WithDeliveryRetries set the number of retries at Delivery.
func WithDeliveryRetries(retries int32) DeliveryOption {
	return func(d *eventingv1.DeliverySpec) {
		d.Retry = &retries
	}
}

// WithDeliveryBackoff sets the retries Backoff options at Delivery.
func WithDeliveryBackoff(policy *eventingv1.BackoffPolicyType, delay *string) DeliveryOption {
	return func(d *eventingv1.DeliverySpec) {
		d.BackoffPolicy = policy
		d.BackoffDelay = delay
	}
}

// WithDeliveryDLS sets the dead letter Sink URI option at Delivery.
func WithDeliveryDeadLetterSinkURI(uri *apis.URL) DeliveryOption {
	return func(d *eventingv1.DeliverySpec) {
		if d.DeadLetterSink == nil {
			d.DeadLetterSink = &v1.Destination{}
		}
		d.DeadLetterSink.URI = uri
	}
}

// WithDeliveryDeadLetterSinkRef sets the DLS URI option at Delivery.
func WithDeliveryDeadLetterSinkRef(ref *v1.KReference) DeliveryOption {
	return func(d *eventingv1.DeliverySpec) {
		if d.DeadLetterSink == nil {
			d.DeadLetterSink = &v1.Destination{}
		}
		d.DeadLetterSink.Ref = ref
	}
}

// WithDeliveryTimeout sets the dead letter Sink option at Delivery.
func WithDeliveryTimeout(timeout *string) DeliveryOption {
	return func(d *eventingv1.DeliverySpec) {
		d.Timeout = timeout
	}
}
