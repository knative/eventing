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
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"

	"github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeFeed(channelDNS string, flow *v1alpha1.Flow) *feedsv1alpha1.Feed {
	feed := &feedsv1alpha1.Feed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: flow.Name + "-",
			Namespace:    flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				// Ideally this would block owner deletion, to ensure the Flow is only deleted
				// after the Feed is gone, but because EventTypes are also owners of the Feed,
				// it results in the Feed not getting cleaned up and the Flow not getting cleaned
				// up. Once EventTypes are no longer owners of the Feed, then this should
				// blockOwnerDeletion.
				*controller.NewControllerRef(flow),
			},
		},
		Spec: feedsv1alpha1.FeedSpec{
			Action: feedsv1alpha1.FeedAction{DNSName: channelDNS},
			Trigger: feedsv1alpha1.EventTrigger{
				EventType: flow.Spec.Trigger.EventType,
				Resource:  flow.Spec.Trigger.Resource,
				Service:   flow.Spec.Trigger.Service,
			},
		},
	}
	if flow.Spec.ServiceAccountName != "" {
		feed.Spec.ServiceAccountName = flow.Spec.ServiceAccountName
	}

	if flow.Spec.Trigger.Parameters != nil {
		feed.Spec.Trigger.Parameters = flow.Spec.Trigger.Parameters
	}
	if flow.Spec.Trigger.ParametersFrom != nil {
		feed.Spec.Trigger.ParametersFrom = flow.Spec.Trigger.ParametersFrom
	}
	return feed
}
