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

package resources

import (
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"github.com/knative/eventing/pkg/controller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeSubscription(channelName string, target string, flow *v1alpha1.Flow) *channelsv1alpha1.Subscription {
	subscriptionName := flow.Name
	subscription := &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscriptionName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*controller.NewControllerRef(flow, false),
			},
		},
		Spec: channelsv1alpha1.SubscriptionSpec{
			Channel:    channelName,
			Subscriber: target,
		},
	}
	return subscription
}
