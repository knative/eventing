/*
Copyright 2018 The Knative Authors

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

package util

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// GcpPubSubChannelStatus is the struct saved to Channel's status.raw if the Channel's provisioner
// is gcp-pubsub. It is used to send data to the dispatcher from the controller.
type GcpPubSubChannelStatus struct {
	// Secret is the Secret that contains the credential to use.
	Secret *corev1.ObjectReference `json:"secret"`
	// SecretKey is the key in Secret that contains the credential to use.
	SecretKey string `json:"secretKey"`

	// GCPProject is the GCP project where the Topic and Subscription exist.
	GCPProject string `json:"gcpProject"`
	// Topic is the name of the PubSub Topic created in GCP to represent this Channel.
	Topic string `json:"topic,omitempty"`
	// Subscriptions is the list of Subscriptions to this Channel and the PubSub Subscription in GCP
	// that represents the Knative Eventing Subscription.
	Subscriptions []GcpPubSubSubscriptionStatus `json:"subscriptions,omitempty"`
}

// GcpPubSubSubscriptionStatus represents the saved status of a gcp-pubsub Channel.
type GcpPubSubSubscriptionStatus struct {
	// Ref is a reference to the Knative Eventing Subscription that this status represents.
	// +optional
	Ref *corev1.ObjectReference `json:"ref,omitempty"`
	// SubscriberURI is a copy of the SubscriberURI of this Subscription.
	// +optional
	SubscriberURI string `json:"subscriberURI,omitempty"`
	// ReplyURI is a copy of the ReplyURI of this Subscription.
	// +optional
	ReplyURI string `json:"replyURI,omitempty"`

	// Subscription is the name of the PubSub Subscription resource in GCP that represents this
	// Knative Eventing Subscription.
	Subscription string `json:"subscription,omitempty"`
}

// IsEmpty determines if this GcpPubSubChannelStatus is equivalent to &GcpPubSubChannelStatus{}. It
// exists because slices are not compared by golang's ==.
func (pbs *GcpPubSubChannelStatus) IsEmpty() bool {
	if pbs.Secret != nil {
		return false
	}
	if pbs.SecretKey != "" {
		return false
	}
	if pbs.GCPProject != "" {
		return false
	}
	if pbs.Topic != "" {
		return false
	}
	if len(pbs.Subscriptions) > 0 {
		return false
	}
	return true
}

// SaveRawStatus saves GcpPubSubChannelStatus to the given Channel, which should only be one whose
// provisioner is gcp-pubsub.
func SaveRawStatus(ctx context.Context, c *eventingv1alpha1.Channel, pbs *GcpPubSubChannelStatus) error {
	jb, err := json.Marshal(pbs)
	if err != nil {
		logging.FromContext(ctx).Error("Error saving the raw status", zap.Error(err), zap.Any("pbs", pbs))
		return err
	}
	c.Status.Raw = &runtime.RawExtension{
		Raw: jb,
	}
	return nil
}

// ReadRawStatus reads GcpPubSubChannelStatus from the given Channel, which should only be one whose
// provisioner is gcp-pubsub. If the raw status is not set, then the empty GcpPubSubChannelStatus is
// returned.
func ReadRawStatus(ctx context.Context, c *eventingv1alpha1.Channel) (*GcpPubSubChannelStatus, error) {
	if c.Status.Raw == nil {
		return &GcpPubSubChannelStatus{}, nil
	}
	bytes := c.Status.Raw.Raw
	if len(bytes) == 0 {
		return &GcpPubSubChannelStatus{}, nil
	}
	var pbs GcpPubSubChannelStatus
	if err := json.Unmarshal(bytes, &pbs); err != nil {
		logging.FromContext(ctx).Error("Unable to parse the raw status", zap.Error(err))
		return nil, err
	}
	return &pbs, nil

}
