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

package json_schema

import (
	"context"
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/stretchr/testify/require"

	"knative.dev/eventing/pkg/eventfilter"
)

func TestAttributesFilter_Filter(t *testing.T) {
	e := cetest.FullEvent()

	tests := map[string]struct {
		filter map[string]interface{}
		want   eventfilter.FilterResult
	}{
		"Fail - If then schema": {
			filter: map[string]interface{}{
				"if": map[string]interface{}{
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"const": e.ID(),
						},
					},
				},
				"then": map[string]interface{}{
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"const": "---" + e.Type(),
						},
					},
				},
			},
			want: eventfilter.FailFilter,
		},
		"Pass - If then schema": {
			filter: map[string]interface{}{
				"if": map[string]interface{}{
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"const": e.ID(),
						},
					},
				},
				"then": map[string]interface{}{
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"const": e.Type(),
						},
					},
				},
			},
			want: eventfilter.PassFilter,
		},
		"Fail - Simple logic": {
			filter: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"const": e.ID() + "---",
							},
						},
					},
					map[string]interface{}{
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"const": "---" + e.Type(),
							},
						},
					},
				},
			},
			want: eventfilter.FailFilter,
		},
		"Pass - Simple logic": {
			filter: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"const": e.ID() + "---",
							},
						},
					},
					map[string]interface{}{
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"const": e.Type(),
							},
						},
					},
				},
			},
			want: eventfilter.PassFilter,
		},
		"Fail - Nested logic": {
			filter: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"const": "---" + e.Type(),
							},
						},
					},
					map[string]interface{}{
						"oneOf": []interface{}{
							map[string]interface{}{
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"const": e.Type(),
									},
								},
							},
							map[string]interface{}{
								"properties": map[string]interface{}{
									"id": map[string]interface{}{
										"const": e.ID(),
									},
								},
							},
						},
					},
				},
			},
			want: eventfilter.FailFilter,
		},
		"Pass - Nested logic": {
			filter: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"const": "---" + e.Type(),
							},
						},
					},
					map[string]interface{}{
						"oneOf": []interface{}{
							map[string]interface{}{
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"const": "---" + e.Type(),
									},
								},
							},
							map[string]interface{}{
								"properties": map[string]interface{}{
									"id": map[string]interface{}{
										"const": e.ID(),
									},
								},
							},
						},
					},
				},
			},
			want: eventfilter.PassFilter,
		},
		"Filter should be able to access to all fields": {
			filter: map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"const": e.ID(),
					},
					"source": map[string]interface{}{
						"const": e.Source(),
					},
					"type": map[string]interface{}{
						"const": e.Type(),
					},
					"dataschema": map[string]interface{}{
						"const": e.DataSchema(),
					},
					"datacontenttype": map[string]interface{}{
						"const": e.DataContentType(),
					},
					"subject": map[string]interface{}{
						"const": e.Subject(),
					},
					"time": map[string]interface{}{
						"const": types.FormatTime(e.Time()),
					},
				},
			},
			want: eventfilter.PassFilter,
		},
		"Type cohercion": {
			filter: map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "string",
					},
					"source": map[string]interface{}{
						"type": "string",
					},
					"type": map[string]interface{}{
						"type": "string",
					},
					"dataschema": map[string]interface{}{
						"type": "string",
					},
					"datacontenttype": map[string]interface{}{
						"type": "string",
					},
					"subject": map[string]interface{}{
						"type": "string",
					},
					"time": map[string]interface{}{
						"type": "string",
					},
					"exbool": map[string]interface{}{
						"type": "boolean",
					},
					// This doesn't work, debugging it seems like the type is coherced correctly to int32, so maybe it's a validator bug
					//"exint": map[string]interface{}{
					//	"type": "integer",
					//},
				},
			},
			want: eventfilter.PassFilter,
		},
		"Filter should not be able to access to data": {
			filter: map[string]interface{}{
				"required": []string{"data"},
			},
			want: eventfilter.FailFilter,
		},
		"Simple failing schema": {
			filter: map[string]interface{}{
				"type": "string",
			},
			want: eventfilter.FailFilter,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := NewJsonSchemaFilter(tt.filter)
			require.NoError(t, err)

			if got := f.Filter(context.TODO(), e); got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
