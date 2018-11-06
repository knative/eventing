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
	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
)

func (current *GcpPubSubSource) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*GcpPubSubSource)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a GcpPubSubSource"}
	}
	if original == nil {
		return nil
	}

	// All of the fields are immutable because the controller doesn't understand when it would need
	// to delete and create a new Receive Adapter with updated arguments. We could relax it slightly
	// to allow a nil Sink -> non-nil Sink, but I don't think it is needed yet.
	if diff := cmp.Diff(original.Spec, current.Spec); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
