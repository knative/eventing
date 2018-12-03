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

type GcpPubSubChannelStatus struct {
	Secret    *corev1.ObjectReference `json:"secret"`
	SecretKey string                  `json:"secretKey"`

	GCPProject    string                        `json:"gcpProject"`
	Topic         string                        `json:"topic,omitempty"`
	Subscriptions []GcpPubSubSubscriptionStatus `json:"subscriptions,omitempty"`
}

type GcpPubSubSubscriptionStatus struct {
	// +optional
	Ref *corev1.ObjectReference `json:"ref,omitempty"`
	// +optional
	SubscriberURI string `json:"subscriberURI,omitempty"`
	// +optional
	ReplyURI string `json:"replyURI,omitempty"`

	Subscription string `json:"subscription,omitempty"`
}

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
