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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// DeliverySpec contains the delivery options for event senders,
// such as channelable and source.
type DeliverySpec struct {
	// DeadLetterSink is the sink receiving event that couldn't be sent to
	// a destination.
	// +optional
	DeadLetterSink *duckv1beta1.Destination `json:"deadLetterSink,omitempty"`

	// Retry is the minimum number of retries the sender should attempt when
	// sending an event before moving it to the dead letter sink.
	// +optional
	Retry *int32 `json:"retry,omitempty"`

	// BackoffPolicy is the retry backoff policy (linear, exponential)
	// +optional
	BackoffPolicy *BackoffPolicyType `json:"backoffPolicy,omitempty"`

	// BackoffDelay is the delay before retrying.
	// More information on Duration format: https://www.ietf.org/rfc/rfc3339.txt
	//
	// For linear policy, backoff delay is the time interval between retries.
	// For exponential policy , backoff delay is backoffDelay*2^<numberOfRetries>
	// +optional
	BackoffDelay *string
}

// BackoffPolicyType is the type for backoff policies
type BackoffPolicyType string

const (
	// Linear backoff policy
	BackoffPolicyLinear BackoffPolicyType = "linear"

	// Exponential backoff policy
	BackoffPolicyExponential BackoffPolicyType = "exponential"
)

// DeliveryStatus contains the Status of an object supporting delivery options.
type DeliveryStatus struct {
	// DeadLetterChannel is the reference to the native, platform specific channel
	// where failed events are sent to.
	// +optional
	DeadLetterChannel *corev1.ObjectReference `json:"deadLetterChannel,omitempty"`
}
