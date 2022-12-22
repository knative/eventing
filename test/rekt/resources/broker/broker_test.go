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

package broker_test

import (
	"embed"
	"os"

	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
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
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":        "foo",
		"namespace":   "bar",
		"brokerClass": "a-broker-class",
		"config": map[string]interface{}{
			"kind":       "cfgkind",
			"name":       "cfgname",
			"apiVersion": "cfgapi",
		},
		"delivery": map[string]interface{}{
			"retry":         "42",
			"backoffPolicy": "exponential",
			"backoffDelay":  "2007-03-01T13:00:00Z/P1Y2M10DT2H30M",
			"deadLetterSink": map[string]interface{}{
				"ref": map[string]string{
					"kind":       "deadkind",
					"name":       "deadname",
					"apiVersion": "deadapi",
				},
				"uri": "/extra/path",
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
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	//   annotations:
	//     eventing.knative.dev/broker.class: a-broker-class
	// spec:
	//   config:
	//     kind: cfgkind
	//     namespace: bar
	//     name: cfgname
	//     apiVersion: cfgapi
	//   delivery:
	//     deadLetterSink:
	//       ref:
	//         kind: deadkind
	//         namespace: bar
	//         name: deadname
	//         apiVersion: deadapi
	//       uri: /extra/path
	//     retry: 42
	//     backoffPolicy: exponential
	//     backoffDelay: "2007-03-01T13:00:00Z/P1Y2M10DT2H30M"
}

func ExampleWithBrokerClass() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	broker.WithBrokerClass("a-broker-class")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	//   annotations:
	//     eventing.knative.dev/broker.class: a-broker-class
	// spec:
}

func ExampleWithAnnotations() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	broker.WithAnnotations(map[string]interface{}{
		"eventing.knative.dev/foo": "bar",
	})(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	//   annotations:
	//     eventing.knative.dev/foo: bar
	// spec:
}

func ExampleWithDeadLetterSink() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	broker.WithDeadLetterSink(&v1.KReference{
		Kind:       "deadkind",
		Name:       "deadname",
		APIVersion: "deadapi",
	}, "/extra/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   delivery:
	//     deadLetterSink:
	//       ref:
	//         kind: deadkind
	//         namespace: bar
	//         name: deadname
	//         apiVersion: deadapi
	//       uri: /extra/path
}

func ExampleWithConfig() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	broker.WithConfig("my-funky-config")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   config:
	//     kind: ConfigMap
	//     namespace: bar
	//     name: my-funky-config
	//     apiVersion: v1
}

func ExampleWithConfigNamespace() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	broker.WithConfigNamespace("knative-eventing")(cfg)
	broker.WithConfig("my-funky-config")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   config:
	//     kind: ConfigMap
	//     namespace: knative-eventing
	//     name: my-funky-config
	//     apiVersion: v1
}

func ExampleWithRetry() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	exp := eventingv1.BackoffPolicyExponential
	broker.WithRetry(42, &exp, ptr.String("2007-03-01T13:00:00Z/P1Y2M10DT2H30M"))(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   delivery:
	//     retry: 42
	//     backoffPolicy: exponential
	//     backoffDelay: "2007-03-01T13:00:00Z/P1Y2M10DT2H30M"
}

func ExampleWithRetry_onlyCount() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	broker.WithRetry(42, nil, nil)(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   delivery:
	//     retry: 42
}
