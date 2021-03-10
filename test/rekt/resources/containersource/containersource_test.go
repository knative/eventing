/*
Copyright 2021 The Knative Authors

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

package containersource_test

import (
	"os"

	"knative.dev/reconciler-test/pkg/manifest"
)

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"sink": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "sinkkind",
				"namespace":  "sinknamespace",
				"name":       "sinkname",
				"apiVersion": "sinkversion",
			},
		},
		"image": "this-image",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: ContainerSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   template:
	//     spec:
	//       containers:
	//         - image: this-image
	//           name: user-container
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: bar
	//       name: sinkname
	//       apiVersion: sinkversion
}

func Example_full() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"sink": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "sinkkind",
				"namespace":  "sinknamespace",
				"name":       "sinkname",
				"apiVersion": "sinkversion",
			},
			"uri": "uri/parts",
		},
		"image": "this-image",
		"args": []string{
			"arg set 1",
			"arg set 2",
		},
		"env": map[string]string{
			"env1": "value1",
			"env2": "value2",
		},
		"ceOverrides": map[string]interface{}{
			"extensions": map[string]string{
				"ext1": "value1",
				"ext2": "value2",
			},
		},
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: ContainerSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   ceOverrides:
	//     extensions:
	//       ext1: value1
	//       ext2: value2
	//   template:
	//     spec:
	//       containers:
	//         - image: this-image
	//           name: user-container
	//           args:
	//             - arg set 1
	//             - arg set 2
	//           env:
	//             - name: env1
	//               value: value1
	//             - name: env2
	//               value: value2
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: bar
	//       name: sinkname
	//       apiVersion: sinkversion
	//     uri: uri/parts
}
