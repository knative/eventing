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

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

func ParallelFilterSubscriptionName(parallelName string, branchNumber int) string {
	return fmt.Sprintf("%s-kn-parallel-filter-%d", parallelName, branchNumber)
}

func ParallelSubscriptionName(parallelName string, branchNumber int) string {
	return fmt.Sprintf("%s-kn-parallel-%d", parallelName, branchNumber)
}

func NewFilterSubscription(branchNumber int, p *v1.Parallel) *messagingv1.Subscription {
	r := &messagingv1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "messaging.knative.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ParallelFilterSubscriptionName(p.Name, branchNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ParallelChannelName(p.Name),
			},
		},
	}
	if p.Spec.Branches[branchNumber].Filter != nil {
		r.Spec.Subscriber = &duckv1.Destination{
			Ref: p.Spec.Branches[branchNumber].Filter.Ref,
			URI: p.Spec.Branches[branchNumber].Filter.URI,
		}
	}
	r.Spec.Reply = &duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: p.Spec.ChannelTemplate.APIVersion,
			Kind:       p.Spec.ChannelTemplate.Kind,
			Name:       ParallelBranchChannelName(p.Name, branchNumber),
			Namespace:  p.Namespace,
		},
	}
	return r
}

func NewSubscription(branchNumber int, p *v1.Parallel) *messagingv1.Subscription {
	r := &messagingv1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "messaging.knative.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ParallelSubscriptionName(p.Name, branchNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ParallelBranchChannelName(p.Name, branchNumber),
			},
			Subscriber: &duckv1.Destination{
				Ref: p.Spec.Branches[branchNumber].Subscriber.Ref,
				URI: p.Spec.Branches[branchNumber].Subscriber.URI,
			},
			Delivery: p.Spec.Branches[branchNumber].Delivery,
		},
	}

	if p.Spec.Branches[branchNumber].Reply != nil {
		r.Spec.Reply = &duckv1.Destination{
			Ref: p.Spec.Branches[branchNumber].Reply.Ref,
			URI: p.Spec.Branches[branchNumber].Reply.URI,
		}
	} else if p.Spec.Reply != nil {
		r.Spec.Reply = &duckv1.Destination{
			Ref: p.Spec.Reply.Ref,
			URI: p.Spec.Reply.URI,
		}
	}
	return r
}
