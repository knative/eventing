/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"encoding/json"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Channel represents a named endpoint on which a Bus accepts event delivery and
// corresponds to the channels.channels.knative.dev CRD. The Bus handles
// provisioning channels, delivering events to Channels, and delivering events
// from Channels to their Subscriptions.
type Channel struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ChannelSpec   `json:"spec"`
	Status             ChannelStatus `json:"status,omitempty"`
}

// ChannelSpec specifies the Bus backing a channel and the configuration
// arguments for the channel.
type ChannelSpec struct {
	// Name of the bus backing this channel (optional)
	Bus string `json:"bus`

	// ClusterBus name of the clusterbus backing this channel (mutually exclusive with Bus)
	ClusterBus string `json:"clusterBus"`

	// Arguments is a list of configuration arguments for the Channel. The
	// Arguments for a channel must contain values for each of the Parameters
	// specified by the Bus' spec.parameters.Channels field except the
	// Parameters that have a default value. If a Parameter has a default value
	// and it is not in the list of Arguments, the default value will be used; a
	// Parameter without a default value that does not have an Argument will
	// result in an error setting up the Channel.
	Arguments *[]Argument `json:"arguments,omitempty"`
}

// ChannelStatus (computed) for a channel
type ChannelStatus struct {
}

func (c *Channel) GetSpecJSON() ([]byte, error) {
	return json.Marshal(c.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList returned in list operations
type ChannelList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Channel `json:"items"`
}
