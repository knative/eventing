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
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		ps   *ClusterProvisionerSpec
		want *apis.FieldError
	}{{
		name: "valid",
		ps: &ClusterProvisionerSpec{
			Type: runtime.TypeMeta{
				APIVersion: "v1beta1",
				Kind:       "Channel",
			},
		},
	}, {
		name: "invalid cluster provisioner, empty type",
		ps: &ClusterProvisionerSpec{
			Type: runtime.TypeMeta{},
		},
		want: apis.ErrMissingField("type"),
	}, {
		name: "invalid cluster provisioner, empty kind",
		ps: &ClusterProvisionerSpec{
			Type: runtime.TypeMeta{
				APIVersion: "v1beta1",
			},
		},
		want: apis.ErrMissingField("type.kind"),
	}, {
		name: "invalid cluster provisioner",
		ps: &ClusterProvisionerSpec{
			Type: runtime.TypeMeta{
				Kind: "Channel",
			},
		},
		want: apis.ErrMissingField("type.apiVersion"),
	}, {
		name: "empty",
		ps:   &ClusterProvisionerSpec{},
		want: apis.ErrMissingField("type"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ps.Validate()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("validate (-want, +got) = %v", diff)
			}
		})
	}
}
