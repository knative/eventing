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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestClusterProvisionerValidate(t *testing.T) {
	tests := []struct {
		name string
		p    *ClusterProvisioner
		want *apis.FieldError
	}{{
		name: "valid",
		p: &ClusterProvisioner{
			Spec: ClusterProvisionerSpec{
				Reconciles: metav1.GroupKind{
					Group: "knative.dev",
					Kind:  "Channel",
				},
			},
		},
	}, {
		name: "invalid cluster provisioner, empty reconciles",
		p: &ClusterProvisioner{
			Spec: ClusterProvisionerSpec{
				Reconciles: metav1.GroupKind{},
			},
		},
		want: apis.ErrMissingField("spec.reconciles"),
	}, {
		name: "invalid cluster provisioner, empty kind",
		p: &ClusterProvisioner{
			Spec: ClusterProvisionerSpec{
				Reconciles: metav1.GroupKind{
					Group: "eventing.knative.test",
				},
			},
		},
		want: apis.ErrMissingField("spec.reconciles.kind"),
	}, {
		name: "invalid cluster provisioner",
		p: &ClusterProvisioner{
			Spec: ClusterProvisionerSpec{
				Reconciles: metav1.GroupKind{
					Kind: "Channel",
				},
			},
		},
		want: apis.ErrMissingField("spec.reconciles.group"),
	}, {
		name: "empty",
		p:    &ClusterProvisioner{},
		want: apis.ErrMissingField("spec.reconciles"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.p.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("validate (-want, +got) = %v", diff)
			}
		})
	}
}
