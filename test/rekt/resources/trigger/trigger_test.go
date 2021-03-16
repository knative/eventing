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
	"os"

	"knative.dev/reconciler-test/pkg/manifest"

	v1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/test/rekt/resources/trigger"
)

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
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

	files, err := manifest.ExecuteLocalYAML(images, cfg)
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
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
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

	files, err := manifest.ExecuteLocalYAML(images, cfg)
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
	//       type: z
	//       x: y
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithSubscriber() {
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

	files, err := manifest.ExecuteLocalYAML(images, cfg)
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

	files, err := manifest.ExecuteLocalYAML(images, cfg)
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
	//       type: z
	//       x: y
}
