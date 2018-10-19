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
	"testing"
)

func TestChannelSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *ChannelSpec
		want *apis.FieldError
	}{{
		name: "namespace valid",
		c: &ChannelSpec{
			Bus: "foo",
		},
		want: nil,
	}, {
		name: "cluster valid",
		c: &ChannelSpec{
			ClusterBus: "bar",
		},
		want: nil,
	}, {
		name: "empty",
		c:    &ChannelSpec{},
		want: apis.ErrMissingOneOf("bus", "clusterBus"),
	}, {
		name: "mutually exclusive missing",
		c: &ChannelSpec{
			Arguments: &[]Argument{{Name: "foo", Value: "bar"}},
		},
		want: apis.ErrMissingOneOf("bus", "clusterBus"),
	}, {
		name: "mutually exclusive both",
		c: &ChannelSpec{
			Bus:        "foo",
			ClusterBus: "bar",
		},
		want: apis.ErrMultipleOneOf("bus", "clusterBus"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("validateChannel (-want, +got) = %v", diff)
			}
		})
	}
}
