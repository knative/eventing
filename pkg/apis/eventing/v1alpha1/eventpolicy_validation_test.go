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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

func TestEventPolicySpecValidation(t *testing.T) {
	tests := []struct {
		name string
		ep   *EventPolicy
		want *apis.FieldError
	}{
		{
			name: "valid, empty",
			ep: &EventPolicy{
				Spec: EventPolicySpec{},
			},
			want: func() *apis.FieldError {
				return nil
			}(),
		},
		{
			name: "invalid, missing from.ref and from.sub",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					From: []EventPolicySpecFrom{{}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingOneOf("ref", "sub").ViaFieldIndex("from", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, from.ref missing name",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					From: []EventPolicySpecFrom{{
						Ref: &EventPolicyFromReference{
							APIVersion: "a",
							Kind:       "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("name").ViaField("ref").ViaFieldIndex("from", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, from.ref missing kind",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					From: []EventPolicySpecFrom{{
						Ref: &EventPolicyFromReference{
							APIVersion: "a",
							Name:       "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("kind").ViaField("ref").ViaFieldIndex("from", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, from.ref missing apiVersion",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					From: []EventPolicySpecFrom{{
						Ref: &EventPolicyFromReference{
							Kind: "a",
							Name: "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("apiVersion").ViaField("ref").ViaFieldIndex("from", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, bot from.ref and from.sub set",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					From: []EventPolicySpecFrom{{
						Ref: &EventPolicyFromReference{
							APIVersion: "a",
							Kind:       "b",
							Name:       "c",
						},
						Sub: ptr.String("abc"),
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMultipleOneOf("ref", "sub").ViaFieldIndex("from", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, missing to.ref and to.selector",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					To: []EventPolicySpecTo{{}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingOneOf("ref", "selector").ViaFieldIndex("to", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, both to.ref and to.selector set",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					To: []EventPolicySpecTo{
						{
							Ref: &EventPolicyToReference{
								APIVersion: "a",
								Kind:       "b",
								Name:       "c",
							},
							Selector: &EventPolicySelector{},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMultipleOneOf("ref", "selector").ViaFieldIndex("to", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, to.ref missing name",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					To: []EventPolicySpecTo{{
						Ref: &EventPolicyToReference{
							APIVersion: "a",
							Kind:       "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("name").ViaField("ref").ViaFieldIndex("to", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, to.ref missing kind",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					To: []EventPolicySpecTo{{
						Ref: &EventPolicyToReference{
							APIVersion: "a",
							Name:       "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("kind").ViaField("ref").ViaFieldIndex("to", 0).ViaField("spec")
			}(),
		},
		{
			name: "invalid, to.ref missing apiVersion",
			ep: &EventPolicy{
				Spec: EventPolicySpec{
					To: []EventPolicySpecTo{{
						Ref: &EventPolicyToReference{
							Kind: "a",
							Name: "b",
						},
					}},
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrMissingField("apiVersion").ViaField("ref").ViaFieldIndex("to", 0).ViaField("spec")
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ep.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventPolicySpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
