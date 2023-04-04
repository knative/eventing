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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var validationTests = []struct {
	name string
	ref  duckv1.KReference
	want *apis.FieldError
}{
	{
		name: "valid kref",
		ref: duckv1.KReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "MyChannel",
		},
		want: nil,
	},
	{
		name: "missing kind",
		ref: duckv1.KReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "",
		},
		want: apis.ErrMissingField("kind"),
	},
	{
		name: "contains namespace",
		ref: duckv1.KReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "MyChannel",
			Namespace:  "my-namespace",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	},
}

func TestIsChannelEmpty(t *testing.T) {
	name := "non empty"
	t.Run(name, func(t *testing.T) {
		r := duckv1.KReference{
			Name:       "boaty-mcboatface",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Channel",
		}
		if isChannelEmpty(r) {
			t.Errorf("%s: isChannelEmpty(%+v) should be false", name, r)
		}
	})

	name = "empty"
	t.Run(name, func(t *testing.T) {
		r := duckv1.KReference{}
		if !isChannelEmpty(r) {
			t.Errorf("%s: isChannelEmpty(%+v) should be true", name, r)
		}
	})
}

func TestIsValidChannel(t *testing.T) {
	for _, test := range validationTests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidChannel(context.TODO(), test.ref)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validation (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
