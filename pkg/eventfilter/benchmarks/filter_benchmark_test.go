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

package benchmarks

import (
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/json_schema"
)

func BenchmarkJsonSchemaFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			f, _ := json_schema.NewJsonSchemaFilter(i.(map[string]interface{}))
			return f
		},
		FilterBenchmark{
			name: "Pass with exact match of id",
			arg: map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"const": event.ID(),
					},
				},
			},
			event: event,
		},
		FilterBenchmark{
			name: "Pass with exact match of all context attributes (except time)",
			arg: map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"const": event.ID(),
					},
					"source": map[string]interface{}{
						"const": event.Source(),
					},
					"type": map[string]interface{}{
						"const": event.Type(),
					},
					"dataschema": map[string]interface{}{
						"const": event.DataSchema(),
					},
					"datacontenttype": map[string]interface{}{
						"const": event.DataContentType(),
					},
					"subject": map[string]interface{}{
						"const": event.Subject(),
					},
				},
			},
			event: event,
		},
		FilterBenchmark{
			name: "No pass with exact match of id and source",
			arg: map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"const": "qwertyuiopasdfghjklzxcvbnm",
					},
					"source": map[string]interface{}{
						"const": "qwertyuiopasdfghjklzxcvbnm",
					},
				},
			},
			event: event,
		},
		FilterBenchmark{
			name: "No pass with if then",
			arg: map[string]interface{}{
				"if": map[string]interface{}{
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"const": event.ID(),
						},
					},
				},
				"then": map[string]interface{}{
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"const": "---" + event.Type(),
						},
					},
				},
			},
			event: event,
		},
		FilterBenchmark{
			name: "No pass with nested logic",
			arg: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"const": "---" + event.Type(),
							},
						},
					},
					map[string]interface{}{
						"oneOf": []interface{}{
							map[string]interface{}{
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"const": event.Type(),
									},
								},
							},
							map[string]interface{}{
								"properties": map[string]interface{}{
									"id": map[string]interface{}{
										"const": event.ID(),
									},
								},
							},
						},
					},
				},
			},
			event: event,
		},
	)
}
