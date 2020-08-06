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
	"fmt"

	"knative.dev/pkg/kmeta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func SequenceSubscriptionName(sequenceName string, step int) string {
	return fmt.Sprintf("%s-kn-sequence-%d", sequenceName, step)
}

func NewSubscription(stepNumber int, s *v1.Sequence) *messagingv1.Subscription {
	r := &messagingv1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "messaging.knative.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      SequenceSubscriptionName(s.Name, stepNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(s),
			},
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: s.Spec.ChannelTemplate.APIVersion,
				Kind:       s.Spec.ChannelTemplate.Kind,
				Name:       SequenceChannelName(s.Name, stepNumber),
			},
			Subscriber: &duckv1.Destination{
				Ref: s.Spec.Steps[stepNumber].Destination.Ref,
				URI: s.Spec.Steps[stepNumber].Destination.URI,
			},
			Delivery: s.Spec.Steps[stepNumber].Delivery,
		},
	}
	// If it's not the last step, use the next channel as the reply to, if it's the very
	// last one, we'll use the (optional) reply from the Sequence Spec.
	if stepNumber < len(s.Spec.Steps)-1 {
		r.Spec.Reply = &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: s.Spec.ChannelTemplate.APIVersion,
				Kind:       s.Spec.ChannelTemplate.Kind,
				Name:       SequenceChannelName(s.Name, stepNumber+1),
				Namespace:  s.Namespace,
			},
		}
	} else if s.Spec.Reply != nil {
		r.Spec.Reply = &duckv1.Destination{
			Ref: s.Spec.Reply.Ref,
			URI: s.Spec.Reply.URI,
		}
	}
	return r
}
