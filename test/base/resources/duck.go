/*
Copyright 2019 The Knative Authors

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

package resources

import (
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// DeliveryOption enables further configuration of DeliverySpec.
type DeliveryOption func(*eventingduckv1alpha1.DeliverySpec)

// WithDeadLetterSinkForDelivery returns an options that adds a DeadLetterSink for the given DeliverySpec.
func WithDeadLetterSinkForDelivery(name string) DeliveryOption {
	return func(delivery *eventingduckv1alpha1.DeliverySpec) {
		if name != "" {
			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: ServiceRef(name),
			}
		}
	}
}

// Delivery returns a DeliverySpec.
func Delivery(options ...DeliveryOption) *eventingduckv1alpha1.DeliverySpec {
	delivery := &eventingduckv1alpha1.DeliverySpec{}
	for _, option := range options {
		option(delivery)
	}
	return delivery
}
