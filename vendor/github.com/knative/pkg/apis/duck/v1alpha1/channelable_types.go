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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/pkg/apis/duck"
)

// Channelable is the schema for the channelable portion of the spec
// section of the resource.
type Channelable struct {
	// TODO: What is actually required here for Channel spec.
	// This is the list of subscriptions for this channel.
	Subscribers []ChannelSubsciberSpec `json:"subscribers,omitempty"`
}

// ChannelSubscriberSpec defines a single subscriber to a Channel.
// TODO: I think the subscriber contract should be Sinkable. You either
// take it on as your pboblem to deal with, or you reject it. I think
// a subscription should have no knowledge of what happens down the line
// and if there are calls, or results follwoing on. You subsribe to me
// and I give you events and you deal with them??
// of Call or Result must be present.
type ChannelSubscriberSpec struct {
	Sinkable string `json:"sinkable"`
}

// Implementations can verify that they implement Channelable via:
var _ = duck.VerifyType(&Channel{}, &Channelable{})

// Channelable is an Implementable "duck type".
var _ duck.Implementable = (*Channelable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is a skeleton type wrapping Channelable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Channelable ObjectReferences and
// access the Channelable data.  This is not a real resource.
type Channel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ChannelableSpec is the part where Channelable object is
	// configured as to be compatible with Channelable contract.
	Spec ChannelableSpec `json:"spec"`
}

// ChannelableSpec shows how we expect folks to embed Channelable in
// their Spec field.
type ChannelableSpec struct {
	Channelable *Channelable `json:"channelable,omitempty"`
}

// In order for Channelable to be Implementable, Channel must be Populatable.
var _ duck.Populatable = (*Channel)(nil)

// GetFullType implements duck.Implementable
func (_ *Channelable) GetFullType() duck.Populatable {
	return &Channel{}
}

// Populate implements duck.Populatable
func (t *Channel) Populate() {
	t.Spec.Channelable = &Channelable{
		// Populate ALL fields
		Subscriptions: []string{"subscription1", "subscription2"},
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a list of Channel resources
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Channel `json:"items"`
}
