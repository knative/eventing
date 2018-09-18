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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knative/pkg/apis/duck"
)

const (
	StatusKeyName = "subscribable"
)

// Subscribable is the schema for the subscribable portion of the payload
type Subscribable struct {
	// Channelable is a reference to the actual Channelable
	// interface that provides the Subscription capabilities. This may
	// point to object itself or to another object providing the
	// actual implementation.
	Channelable corev1.ObjectReference `json:"channelable,omitempty"`
}

// Implementations can verify that they implement Subscribable via:
var _ = duck.VerifyType(&ChannelableRef{}, &Subscribable{})

// Subscribable is an Implementable "duck type".
var _ duck.Implementable = (*Subscribable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelableRef is a skeleton type wrapping Subscribable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Subscribable ObjectReferences and
// access the Subscribable data.  This is not a real resource.
type ChannelableRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SubscribableStatus is the part of the Status where a Subscribable
	// object points to the underlying Channelable object that fullfills
	// the SubscribableSpec contract. Note that this can be a self-link
	// for example for concrete Channel implementations.
	Status SubscribableStatus `json:"status"`
}

// SubscribableStatus shows how we expect folks to embed Subscribable in
// their Status field.
type SubscribableStatus struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

// In order for Subscribable to be Implementable, ChannelableRef must be Populatable.
var _ duck.Populatable = (*ChannelableRef)(nil)

// GetFullType implements duck.Implementable
func (_ *Subscribable) GetFullType() duck.Populatable {
	return &ChannelableRef{}
}

// Populate implements duck.Populatable
func (t *ChannelableRef) Populate() {
	t.Status.Subscribable = &Subscribable{
		// Populate ALL fields
		Channelable: corev1.ObjectReference{
			Name:       "placeholdername",
			APIVersion: "apiversionhere",
			Kind:       "ChannelKindHere",
		},
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelableRefList is a list of ChannelableRef resources
type ChannelableRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ChannelableRef `json:"items"`
}

func ChannelableRefromUnstructured(obj *unstructured.Unstructured) (*ChannelableRef, error) {
	if obj == nil {
		return nil, nil
	}

	// Use the unstructured marshaller to ensure it's proper JSON
	raw, err := obj.MarshalJSON()
	if err != nil {
		fmt.Printf("Failed to marshal %s : %s\n", string(raw), err)
		return nil, err
	}

	r := &ChannelableRef{}
	if err := json.Unmarshal(raw, r); err != nil {
		fmt.Printf("Failed to unmarshal %s : %s\n", string(raw), err)
		return nil, err
	}
	return r, nil
}
