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

package tracing

import (
	"github.com/cloudevents/sdk-go/pkg/binding"
	"github.com/cloudevents/sdk-go/pkg/binding/transformer"
	"go.opencensus.io/trace"
)

// AddTraceparent returns a CloudEvent that is identical to the input event,
// with the traceparent CloudEvents extension attribute added.
func AddTraceparent(span *trace.Span) []binding.TransformerFactory {
	val := traceparentAttributeValue(span)
	return transformer.SetExtension(traceparentAttribute, val, func(i2 interface{}) (i interface{}, err error) {
		return val, nil
	})
}
