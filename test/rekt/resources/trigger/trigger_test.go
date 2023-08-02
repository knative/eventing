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

package trigger_test

import (
	"embed"
	"os"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/trigger"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

//go:embed *.yaml
var yaml embed.FS

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
		"subscriber": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "subkind",
				"name":       "subname",
				"apiVersion": "subversion",
			},
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
}

func Example_zero() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
		"filter": map[string]interface{}{
			"attributes": map[string]string{
				"x":    "y",
				"type": "z",
			},
		},
		"subscriber": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "subkind",
				"name":       "subname",
				"apiVersion": "subversion",
			},
			"uri": "/extra/path",
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   filter:
	//     attributes:
	//       type: "z"
	//       x: "y"
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithSubscriber() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	trigger.WithSubscriber(&v1.KReference{
		Kind:       "subkind",
		Name:       "subname",
		APIVersion: "subversion",
	}, "/extra/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithFilter() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	trigger.WithFilter(map[string]string{
		"x":    "y",
		"type": "z",
	})(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   filter:
	//     attributes:
	//       type: "z"
	//       x: "y"
}

func ExampleWithDeadLetterSink() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	delivery.WithDeadLetterSink(service.AsKReference("targetdlq"), "/uri/here")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   delivery:
	//     deadLetterSink:
	//       ref:
	//         kind: Service
	//         namespace: bar
	//         name: targetdlq
	//         apiVersion: v1
	//       uri: /uri/here
}

func ExampleWithRetry() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	exp := duckv1.BackoffPolicyExponential
	delivery.WithRetry(3, &exp, ptr.String("T0"))(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   delivery:
	//     retry: 3
	//     backoffPolicy: exponential
	//     backoffDelay: "T0"
}

func ExampleWithNewFilters() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	trigger.WithNewFilters([]eventingv1.SubscriptionsAPIFilter{
		{
			CESQL: "source IN ('order.created', 'order.updated', 'order.canceled')",
		},
		{
			Not: &eventingv1.SubscriptionsAPIFilter{
				CESQL: "type = 'tp'",
			},
		},
	})(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   filters:
	//     - cesql: source IN ('order.created', 'order.updated', 'order.canceled')
	//     - not:
	//         cesql: type = 'tp'
}
