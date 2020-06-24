/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var validationTests = []struct {
	name string
	ref  corev1.ObjectReference
	want *apis.FieldError
}{
	{
		name: "valid object ref",
		ref: corev1.ObjectReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "MyChannel",
		},
		want: nil,
	},
	{
		name: "invalid object ref",
		ref: corev1.ObjectReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "",
		},
		want: apis.ErrMissingField("kind"),
	},
}

func TestIsChannelEmpty(t *testing.T) {
	name := "non empty"
	t.Run(name, func(t *testing.T) {
		r := corev1.ObjectReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Channel",
		}
		if isChannelEmpty(r) {
			t.Errorf("%s: isChannelEmpty(%s) should be false", name, r)
		}
	})

	name = "empty"
	t.Run(name, func(t *testing.T) {
		r := corev1.ObjectReference{}
		if !isChannelEmpty(r) {
			t.Errorf("%s: isChannelEmpty(%s) should be true", name, r)
		}
	})
}

func TestIsValidChannel(t *testing.T) {
	for _, test := range validationTests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidChannel(test.ref)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validation (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestIsValidObjectReference(t *testing.T) {
	tests := []struct {
		name string
		ref  corev1.ObjectReference
		want []*apis.FieldError
	}{
		{
			name: "missing api version and kind",
			ref: corev1.ObjectReference{
				Name:       "boaty-mcboatface",
				APIVersion: "",
				Kind:       "",
			},
			want: []*apis.FieldError{
				apis.ErrMissingField("apiVersion"),
				apis.ErrMissingField("kind"),
			},
		},
		{
			name: "missing name",
			ref: corev1.ObjectReference{
				Name:       "",
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Strait",
			},
			want: []*apis.FieldError{
				apis.ErrMissingField("name"),
			},
		},
		{
			name: "missing all",
			ref: corev1.ObjectReference{
				Name:       "",
				APIVersion: "",
				Kind:       "",
			},
			want: []*apis.FieldError{
				apis.ErrMissingField("name"),
				apis.ErrMissingField("apiVersion"),
				apis.ErrMissingField("kind"),
			},
		},
		{
			name: "missing none",
			ref: corev1.ObjectReference{
				Name:       "kind",
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Channel",
			},
			want: []*apis.FieldError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allWanted := &apis.FieldError{}
			for _, fe := range test.want {
				allWanted = allWanted.Also(fe)
			}
			got := IsValidObjectReference(test.ref)
			if diff := cmp.Diff(allWanted.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validation (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
