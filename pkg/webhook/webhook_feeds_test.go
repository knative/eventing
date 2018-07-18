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

package webhook

import (
	"github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testFeedName             = "test-feed"
	testClusterEventTypeName = "test-clustereventtype"
	testEventTypeName        = "test-eventtype"
)

func createFeed(feedName, eventTypeName, clusterEventTypeName string) v1alpha1.Feed {
	return v1alpha1.Feed{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      feedName,
		},
		Spec: v1alpha1.FeedSpec{
			Trigger: v1alpha1.EventTrigger{
				EventType:        eventTypeName,
				ClusterEventType: clusterEventTypeName,
			},
			// TODO(nicholss): Fill this out more?
		},
	}
}
