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
	"embed"
	"os"

	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Example_min() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"args":      "--period=1",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	//   sink:
	//   template:
	//     spec:
	//       containers:
	//       - name: heartbeats
	//         image: ko://knative.dev/eventing/test/test_images/heartbeats
	//         imagePullPolicy: IfNotPresent
	//         args:
	//         - --period=1
	//         env:
	//         - name: POD_NAME
	//           value: heartbeats
	//         - name: POD_NAMESPACE
	//           value: bar
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"args":      "--period=1",
		"ceOverrides": map[string]interface{}{
			"extensions": map[string]string{
				"ext1": "val1",
				"ext2": "val2",
			},
		},
		"sink": map[string]interface{}{
			"ref": map[string]interface{}{
				"kind":       "AKind",
				"apiVersion": "something.valid/v1",
				"name":       "thesink",
			},
			"uri": "uri/parts",
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	//       ext1: val1
	//       ext2: val2
	//   sink:
	//     ref:
	//       kind: AKind
	//       namespace: bar
	//       name: thesink
	//       apiVersion: something.valid/v1
	//     uri: uri/parts
	//   template:
	//     spec:
	//       containers:
	//       - name: heartbeats
	//         image: ko://knative.dev/eventing/test/test_images/heartbeats
	//         imagePullPolicy: IfNotPresent
	//         args:
	//         - --period=1
	//         env:
	//         - name: POD_NAME
	//           value: heartbeats
	//         - name: POD_NAMESPACE
	//           value: bar
}
