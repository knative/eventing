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

	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSourceValidate(t *testing.T) {
	tests := []CRDTest{{
		name: "empty",
		cr: &Source{
			Spec: SourceSpec{},
		},
		want: apis.ErrMissingField("spec.provisioner"),
	}, {
		name: "minimum valid",
		cr: &Source{
			Spec: SourceSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
	}, {
		name: "full valid",
		cr: &Source{
			Spec: SourceSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"foo":"baz"}`),
				},
				Channel: &corev1.ObjectReference{
					Name:       "bar",
					Kind:       "Channel",
					APIVersion: "eventing.knative.dev/v1alpha1",
				},
			},
		},
	}, {
		name: "invalid, set extra channel parameters",
		cr: &Source{
			Spec: SourceSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Channel: &corev1.ObjectReference{
					Name:            "bar",
					Kind:            "Channel",
					APIVersion:      "eventing.knative.dev/v1alpha1",
					ResourceVersion: "SoMad",
				},
			},
		},
		want: &apis.FieldError{
			Message: "must not set the field(s)",
			Paths:   []string{"spec.channel.ResourceVersion"},
			Details: "only name, apiVersion and kind are supported fields",
		},
	}, {
		name: "invalid, set extra channel as not a channel",
		cr: &Source{
			Spec: SourceSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Channel: &corev1.ObjectReference{
					Name:       "backwards",
					Kind:       "lennahC",
					APIVersion: "eventing.knative.dev/v1alpha1",
				},
			},
		},
		want: &apis.FieldError{
			Message: "invalid value \"lennahC\"",
			Paths:   []string{"spec.channel.kind"},
			Details: "only 'Channel' kind is allowed",
		},
	}}

	doValidateTest(t, tests)
}
