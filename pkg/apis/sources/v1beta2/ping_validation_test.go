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

package v1beta2

import (
	"context"
	"encoding/base64"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestPingSourceValidation(t *testing.T) {
	tests := []struct {
		name   string
		source PingSource
		want   *apis.FieldError
	}{
		{
			name: "valid spec",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule: "*/2 * * * *",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: nil,
		}, {
			name: "valid spec with timezone",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule: "*/2 * * * *",
					Timezone: "Europe/Paris",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: nil,
		}, {
			name: "valid spec with invalid timezone",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule: "*/2 * * * *",
					Timezone: "Knative/Land",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrInvalidValue("provided bad location Knative/Land: unknown time zone Knative/Land", "spec.timezone")
				errs = errs.Also(fe)
				return errs
			}(),
		}, {
			name: "empty sink",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule: "*/2 * * * *",
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrGeneric("expected at least one, got none", "ref", "uri").ViaField("spec.sink")
			}(),
		}, {
			name: "invalid schedule",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule: "2",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrInvalidValue("expected exactly 5 fields, found 1: [2]", "spec.schedule")
				errs = errs.Also(fe)
				return errs
			}(),
		}, {
			name: "valid spec with data",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: "application/json",
					Data:        `{"msg": "Hello World"}`,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: nil,
		}, {
			name: "valid spec with dataBase64",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: cloudevents.TextPlain,
					DataBase64:  "SGVsbG8gV29ybGQu",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: nil,
		}, {
			name: "invalid spec with data and dataBase64 both set",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: cloudevents.TextPlain,
					Data:        "Hello world",
					DataBase64:  "SGVsbG8gV29ybGQu",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrMultipleOneOf("spec.data", "spec.dataBase64")
				errs = errs.Also(fe)
				return errs
			}(),
		}, {
			name: "invalid spec: dataBase64 is invalid base64 string",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: cloudevents.TextPlain,
					DataBase64:  "$$$ invalid base64 string $$$",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrInvalidValue("illegal base64 data at input byte 0", "spec.dataBase64")
				errs = errs.Also(fe)
				return errs
			}(),
		}, {
			name: "invalid spec: contentType=application/json, data is invalid JSON format",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: cloudevents.ApplicationJSON,
					Data:        "Invalid JSON",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrInvalidValue("invalid character 'I' looking for beginning of value", "spec.data")
				errs = errs.Also(fe)
				return errs
			}(),
		}, {
			name: "invalid spec: contentType=application/json, decoded dataBase64 is invalid JSON format",
			source: PingSource{
				Spec: PingSourceSpec{
					Schedule:    "*/2 * * * *",
					ContentType: cloudevents.ApplicationJSON,
					DataBase64:  base64.StdEncoding.EncodeToString([]byte("Invalid JSON")),
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrInvalidValue("invalid character 'I' looking for beginning of value", "spec.dataBase64")
				errs = errs.Also(fe)
				return errs
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.source.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("PingSourceSpec.Validate (-want, +got) =", diff)
			}
		})
	}
}
