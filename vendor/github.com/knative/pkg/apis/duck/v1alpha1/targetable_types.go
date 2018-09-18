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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knative/pkg/apis/duck"
)

// Targetable is the schema for the targetable portion of the payload
// It would be better to have one level of indirection from Status.
// The way this is currently put in at the same level as the other Status
// resources. Hence the objects supporting Targetable (Knative Route for
// example), will expose it as: Status.DomainInternal
// For new resources that are built from scratch, they should probably
// introduce an extra level of indirection.
type Targetable struct {
	DomainInternal string `json:"domainInternal,omitempty"`
}

// Implementations can verify that they implement Targetable via:
var _ = duck.VerifyType(&Target{}, &Targetable{})

// Targetable is an Implementable "duck type".
var _ duck.Implementable = (*Targetable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Target is a skeleton type wrapping Targetable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Targetable ObjectReferences and
// access the Targetable data.  This is not a real resource.
type Target struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status Targetable `json:"status"`
}

// In order for Targetable to be Implementable, Target must be Populatable.
var _ duck.Populatable = (*Target)(nil)

// GetFullType implements duck.Implementable
func (_ *Targetable) GetFullType() duck.Populatable {
	return &Target{}
}

// Populate implements duck.Populatable
func (t *Target) Populate() {
	t.Status = Targetable{
		// Populate ALL fields
		DomainInternal: "this is not empty",
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TargetList is a list of Target resources
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Target `json:"items"`
}

func TargetableFromUnstructured(obj *unstructured.Unstructured) (*Target, error) {
	if obj == nil {
		return nil, nil
	}

	// Use the unstructured marshaller to ensure it's proper JSON
	raw, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	r := &Target{}
	if err := json.Unmarshal(raw, r); err != nil {
		return nil, err
	}
	return r, nil
}
