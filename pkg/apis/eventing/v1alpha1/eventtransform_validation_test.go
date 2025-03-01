/*
Copyright 2025 The Knative Authors

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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/webhook/json"
)

func TestJSONDecode(t *testing.T) {

	et := &EventTransform{}

	err := json.Decode([]byte(`
{
  "apiVersion": "eventing.knative.dev/v1alpha1",
  "kind": "EventTransform",
  "metadata": {
    "name": "identity"
  },
  "spec": {
    "jsonata": {
      "expression": "{\n  \"specversion\": \"1.0\",\n  \"id\": id,\n  \"type\": \"transformation.jsonata\",\n  \"source\": \"transformation.json.identity\",\n  \"data\": $\n}\n"
    }
  }
}
`), et, true)

	assert.Nil(t, err)
}

var sink = &duckv1.Destination{
	URI: apis.HTTP("example.com"),
}

func TestEventTransform_Validate(t *testing.T) {
	tests := []struct {
		name string
		in   EventTransform
		ctx  context.Context
		want *apis.FieldError
	}{
		{
			name: "empty",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec:   EventTransformSpec{},
				Status: EventTransformStatus{},
			},
			ctx:  context.Background(),
			want: apis.ErrMissingOneOf("jsonata").ViaField("spec"),
		},
		{
			name: "jsonata valid",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "1.0" }`,
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx:  context.Background(),
			want: nil,
		},
		{
			name: "jsonata with reply valid",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "1.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "1.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx:  context.Background(),
			want: nil,
		},
		{
			name: "jsonata with reply discard valid",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "1.0" }`,
						},
					},
					Reply: &ReplySpec{
						Discard: ptr.Bool(true),
					},
				},
				Status: EventTransformStatus{},
			},
			ctx:  context.Background(),
			want: nil,
		},
		{
			name: "jsonata with reply, jsonata reply transformations and discard = true, invalid",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "1.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "1.0" }`,
							},
						},
						Discard: ptr.Bool(true),
					},
				},
			},
			ctx:  context.Background(),
			want: (&apis.FieldError{}).Also(apis.ErrMultipleOneOf("jsonata", "discard").ViaField("reply").ViaField("spec")),
		},
		{
			name: "jsonata update change expression",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "2.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx: apis.WithinUpdate(context.Background(), &EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "1.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "1.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			}),
			want: nil,
		},
		{
			name: "transform jsonata change transformation type, have -> not have",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx: apis.WithinUpdate(context.Background(), &EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{},
				},
				Status: EventTransformStatus{},
			}),
			want: (&apis.FieldError{}).
				Also(
					apis.ErrGeneric("Transformations types are immutable, jsonata transformation cannot be changed to a different transformation type. Suggestion: create a new transformation, migrate services to the new one, and delete this transformation.").
						ViaField("jsonata"),
				).
				ViaField("spec"),
		},
		{
			name: "transform jsonata change reply transformation type, have -> not have",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "2.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx: apis.WithinUpdate(context.Background(), &EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
				},
				Status: EventTransformStatus{},
			}),
			want: nil,
		},
		{
			name: "transform jsonata change reply transformation type, jsonata expression -> discard",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					Sink: sink,
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "2.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx: apis.WithinUpdate(context.Background(), &EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
					Reply: &ReplySpec{
						Discard: ptr.Bool(true),
					},
				},
				Status: EventTransformStatus{},
			}),
			want: nil,
		},
		{
			name: "reply without sink",
			in: EventTransform{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: EventTransformSpec{
					EventTransformations: EventTransformations{
						Jsonata: &JsonataEventTransformationSpec{
							Expression: `{ "specversion": "2.0" }`,
						},
					},
					Reply: &ReplySpec{
						EventTransformations: EventTransformations{
							Jsonata: &JsonataEventTransformationSpec{
								Expression: `{ "specversion": "2.0" }`,
							},
						},
					},
				},
				Status: EventTransformStatus{},
			},
			ctx: context.Background(),
			want: (&apis.FieldError{}).Also(
				apis.ErrGeneric("reply is set without spec.sink", "").
					ViaField("reply").
					ViaField("spec"),
			),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.in.Validate(tt.ctx)
			assert.Equalf(t, tt.want, tt.in.Validate(tt.ctx), "Validate(%v) = %v", tt.ctx, got)
		})
	}
}
