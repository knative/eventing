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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var condReady = ChannelCondition{
	Type:   ChannelConditionReady,
	Status: corev1.ConditionTrue,
}

var condUnprovisioned = ChannelCondition{
	Type:   ChannelConditionProvisioned,
	Status: corev1.ConditionFalse,
}

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *ChannelStatus
		condQuery ChannelConditionType
		want      *ChannelCondition
	}{{
		name: "single condition",
		cs: &ChannelStatus{
			Conditions: []ChannelCondition{
				condReady,
			},
		},
		condQuery: ChannelConditionReady,
		want:      &condReady,
	}, {
		name: "multiple conditions",
		cs: &ChannelStatus{
			Conditions: []ChannelCondition{
				condReady,
				condUnprovisioned,
			},
		},
		condQuery: ChannelConditionProvisioned,
		want:      &condUnprovisioned,
	}, {
		name: "unknown condition",
		cs: &ChannelStatus{
			Conditions: []ChannelCondition{
				condReady,
				condUnprovisioned,
			},
		},
		condQuery: ChannelConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelGetSpecJSON(t *testing.T) {
	c := &Channel{
		Spec: ChannelSpec{
			Provisioner: &ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name: "foo",
				},
			},
			Arguments: &runtime.RawExtension{
				Raw: []byte(`{"foo":"baz"}`),
			},
		},
	}

	want := `{"provisioner":{"ref":{"name":"foo"}},"arguments":{"foo":"baz"}}`
	got, err := c.GetSpecJSON()
	if err != nil {
		t.Fatalf("unexpected spec JSON error: %v", err)
	}

	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("unexpected spec JSON (-want, +got) = %v", diff)
	}
}
