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
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"strings"
	"testing"
)

var longName = strings.Repeat("A", 255)

// TODO: add the following tests:
//  1. Multiple parameters to the same Bus.
//  2. Two parameters with the same Name in the same list.
//  3. Multiple errors in the same object.

func TestBusSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		bs   *BusSpec
		want *apis.FieldError
	}{{
		name: "valid",
		bs: &BusSpec{
			Parameters: &BusParameters{
				Channel: &[]Parameter{
					{
						Name:        "foo",
						Description: "bar",
					},
				},
				Subscription: &[]Parameter{
					{
						Name:        "foo",
						Description: "bar",
					},
				},
			},
		},
	}, {
		name: "valid no description",
		bs: &BusSpec{
			Parameters: &BusParameters{
				Channel: &[]Parameter{
					{
						Name: "foo",
					},
				},
				Subscription: &[]Parameter{
					{
						Name: "foo",
					},
				},
			},
		},
	}, {
		name: "invalid channel parameter",
		bs: &BusSpec{
			Parameters: &BusParameters{
				Channel: &[]Parameter{
					{
						Name: "foo@bar",
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: `invalid key name "foo@bar"`,
			Paths: []string{
				"parameters.channel[0].name",
			},
			Details: "a valid config key must consist of alphanumeric characters, '-', '_' or '.' (e.g. 'key.name',  or 'KEY_NAME',  or 'key-name', regex used for validation is '[-._a-zA-Z0-9]+')",
		},
	}, {
		name: "invalid subscription parameter",
		bs: &BusSpec{
			Parameters: &BusParameters{
				Subscription: &[]Parameter{
					{
						Name: "foo@bar",
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: `invalid key name "foo@bar"`,
			Paths: []string{
				"parameters.subscription[0].name",
			},
			Details: "a valid config key must consist of alphanumeric characters, '-', '_' or '.' (e.g. 'key.name',  or 'KEY_NAME',  or 'key-name', regex used for validation is '[-._a-zA-Z0-9]+')",
		},
	}, {
		name: "invalid channel too long",
		bs: &BusSpec{
			Parameters: &BusParameters{
				Channel: &[]Parameter{
					{
						Name: longName,
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: fmt.Sprintf("invalid key name %q", longName),
			Paths: []string{
				"parameters.channel[0].name",
			},
			Details: "must be no more than 253 characters",
		},
	}, {
		name: "empty bus",
		bs:   &BusSpec{},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.bs.Validate()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("validateBus (-want, +got) = %v", diff)
			}
		})
	}
}
