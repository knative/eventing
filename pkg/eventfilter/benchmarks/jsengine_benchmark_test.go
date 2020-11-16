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
	"fmt"
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/jsengine"
)

func BenchmarkJsEngineFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			f, _ := jsengine.NewJsFilter(i.(string))
			return f
		},
		FilterBenchmark{
			name:  "Pass with exact match of id",
			arg:   fmt.Sprintf(`event.id === "%s"`, event.ID()),
			event: event,
		},
		FilterBenchmark{
			name: "Pass with exact match of all context attributes (except time)",
			arg: fmt.Sprintf(
				`event.id === "%s" && event.source === "%s" && event.type === "%s" && event.dataschema === "%s" && event.datacontenttype === "%s" && event.subject === "%s"`,
				event.ID(),
				event.Source(),
				event.Type(),
				event.DataSchema(),
				event.DataContentType(),
				event.Subject(),
			),
			event: event,
		},
		FilterBenchmark{
			name:  "No pass with exact match of id and source",
			arg:   `event.id === "qwertyuiopasdfghjklzxcvbnm" && event.source === "qwertyuiopasdfghjklzxcvbnm"`,
			event: event,
		},
		FilterBenchmark{
			name:  "No pass with if then",
			arg:   fmt.Sprintf(`(event.id === "%s") ? event.type === "---%s" : true`, event.ID(), event.Type()),
			event: event,
		},
		FilterBenchmark{
			name:  "No pass with nested logic",
			arg:   fmt.Sprintf(`(event.type === "---%s") || (event.type === "%s" ? event.id !== "%s" : event.id === "%s")`, event.Type(), event.Type(), event.ID(), event.ID()),
			event: event,
		},
	)
}
