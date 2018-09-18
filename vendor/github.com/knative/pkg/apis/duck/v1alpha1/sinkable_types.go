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

// Sinkable is very similar concept as Targetable. However, at the
// transport level they have different contracts and hence Sinkable
// and Targetable are two distinct resources.
// It would be better to have one level of indirection from Status.
// The way this is currently put in at the same level as the other Status
// resources. Hence the objects supporting Sinkable (Knative channel for
// example), will expose it as: Status.DomainInternal
// For new resources that are built from scratch, they should probably
// introduce an extra level of indirection.
package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knative/pkg/apis/duck"
)

// Sinkable is the schema for the sinkable portion of the payload
type Sinkable struct {
	DomainInternal string `json:"domainInternal,omitempty"`
}

// Implementations can verify that they implement Sinkable via:
var _ = duck.VerifyType(&Sink{}, &Sinkable{})

// Sinkable is an Implementable "duck type".
var _ duck.Implementable = (*Sinkable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Sink is a skeleton type wrapping Sinkable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Sinkable ObjectReferences and
// access the Sinkable data.  This is not a real resource.
type Sink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status Sinkable `json:"status"`
}

// In order for Sinkable to be Implementable, Sink must be Populatable.
var _ duck.Populatable = (*Sink)(nil)

// GetFullType implements duck.Implementable
func (_ *Sinkable) GetFullType() duck.Populatable {
	return &Sink{}
}

// Populate implements duck.Populatable
func (t *Sink) Populate() {
	t.Status = Sinkable{
		// Populate ALL fields
		DomainInternal: "this is not empty",
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SinkList is a list of Sink resources
type SinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Sink `json:"items"`
}

func SinkableFromUnstructured(obj *unstructured.Unstructured) (*Sink, error) {
	if obj == nil {
		return nil, nil
	}

	// Use the unstructured marshaller to ensure it's proper JSON
	raw, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	r := &Sink{}
	if err := json.Unmarshal(raw, r); err != nil {
		return nil, err
	}
	return r, nil
}
