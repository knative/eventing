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

package channel

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/utils"

	eventduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
)

// Underscore is used as a separator so the concatenated version can't accidentally overlap.

// generateTopicName generates the GCP PubSub Topic name given the Knative Channel.
func generateTopicName(c *v1alpha1.Channel) string {
	return utils.TopicNameWithUID("_", c.Name, c.UID)
}

// generateSubName Generates the GCP PubSub Subscription name given the Knative Channel's
// subscriber.
// Note that this requires the subscriber's ref to be set correctly.
func generateSubName(cs *eventduck.ChannelSubscriberSpec) string {
	return utils.TopicNameWithUID("_", cs.Ref.Name, cs.Ref.UID)
}
