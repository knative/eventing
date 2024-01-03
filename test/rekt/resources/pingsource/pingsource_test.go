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

package pingsource_test

import (
	"embed"
	"os"

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
	// apiVersion: sources.knative.dev/v1
	// kind: PingSource
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
		"schedule":    "*/1 * * * *",
		"contentType": "application/json",
		"data":        `{"message": "Hello world!"}`,
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

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: PingSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   schedule: '*/1 * * * *'
	//   contentType: 'application/json'
	//   data: '{"message": "Hello world!"}'
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: sinknamespace
	//       name: sinkname
	//       apiVersion: sinkversion
	//     uri: uri/parts
}

func Example_fullbase64() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":        "foo",
		"namespace":   "bar",
		"schedule":    "*/1 * * * *",
		"contentType": "application/json",
		"dataBase64":  "aabbccddeeff",
		"sink": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "sinkkind",
				"namespace":  "sinknamespace",
				"name":       "sinkname",
				"apiVersion": "sinkversion",
			},
			"uri":     "uri/parts",
			"CACerts": "xyz",
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: PingSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   schedule: '*/1 * * * *'
	//   contentType: 'application/json'
	//   dataBase64: 'aabbccddeeff'
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: sinknamespace
	//       name: sinkname
	//       apiVersion: sinkversion
	//     uri: uri/parts
	//     CACerts: |-
	//       xyz
}

func Example_schedule_with_secs() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":        "foo",
		"namespace":   "bar",
		"schedule":    "10 0/5 * * * ?",
		"contentType": "application/json",
		"data":        `{"message": "Hello world!"}`,
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

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: PingSource
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   schedule: '10 0/5 * * * ?'
	//   contentType: 'application/json'
	//   data: '{"message": "Hello world!"}'
	//   sink:
	//     ref:
	//       kind: sinkkind
	//       namespace: sinknamespace
	//       name: sinkname
	//       apiVersion: sinkversion
	//     uri: uri/parts
}
