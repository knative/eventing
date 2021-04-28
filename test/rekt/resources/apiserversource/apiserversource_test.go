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

package apiserversource_test

import (
	"os"

	"knative.dev/reconciler-test/pkg/manifest"
)

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: ApiServerSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_full() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":               "foo",
		"namespace":          "bar",
		"serviceAccountName": "src-sa",
		"mode":               "src-mode",
		"resources": []map[string]interface{}{{
			"apiVersion": "res1apiVersion",
			"kind":       "res1kind",
		}, {
			"apiVersion": "res2apiVersion",
			"kind":       "res2kind",
		}},
		"sink": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "sinkkind",
				"namespace":  "sinknamespace",
				"name":       "sinkname",
				"apiVersion": "sinkversion",
			},
			"uri": "uri/parts",
		},
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: ApiServerSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   serviceAccountName: src-sa
	//   mode: src-mode
	//   resources:
	//     - apiVersion: res1apiVersion
	//       kind: res1kind
	//     - apiVersion: res2apiVersion
	//       kind: res2kind
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: bar
	//       name: sinkname
	//       apiVersion: sinkversion
	//     uri: uri/parts
}
