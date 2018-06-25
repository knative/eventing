/*
 * Copyright 2018 the original author or authors.
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

// Channel represents the channels.channels.knative.dev CRD
type Channel struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ChannelSpec    `json:"spec"`
	Status             *ChannelStatus `json:"status,omitempty"`
}

// ChannelSpec (what the user wants) for a channel
type ChannelSpec struct {

	// Name of the bus backing this channel (optional)
	Bus string `json:"bus`

	// Arguments configuration arguments for the channel
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
