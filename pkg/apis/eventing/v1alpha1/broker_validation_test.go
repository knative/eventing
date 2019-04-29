/*
Copyright 2019 The Knative Authors
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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
)

func TestBrokerValidation(t *testing.T) {
	name := "invalid policy spec"
	broker := &Broker{Spec: BrokerSpec{}}

	want := &apis.FieldError{
		Paths:   []string{"spec.policy"},
		Message: "missing field(s)",
	}

	t.Run(name, func(t *testing.T) {
		got := broker.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Errorf("Broker.Validate (-want, +got) = %v", diff)
		}
	})
}

func TestBrokerSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		bs   *BrokerSpec
		want *apis.FieldError
	}{{
		name: "invalid broker spec",
		bs:   &BrokerSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("policy")
			return fe
		}(),
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.bs.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate BrokerSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

// No-op test because method does nothing.
func TestBrokerImmutableFields(t *testing.T) {
	original := &Broker{}
	current := &Broker{}
	_ = current.CheckImmutableFields(context.TODO(), original)
}
