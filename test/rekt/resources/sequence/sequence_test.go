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

package sequence_test

import (
	"embed"
	"os"

	v1 "knative.dev/pkg/apis/duck/v1"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/sequence"
)

//go:embed *.yaml
var yaml embed.FS

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
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
	// apiVersion: flows.knative.dev/v1
	// kind: Sequence
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   steps:
}

func Example_fullDelivery() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"channelTemplate": map[string]interface{}{
			"apiVersion": "channelimpl/v1",
			"kind":       "mychannel",
			"spec": map[string]interface{}{
				"delivery": map[string]interface{}{
					"retry": 8,
				},
				"thing2": "value2",
			},
		},
		"steps": []map[string]interface{}{{
			"ref": map[string]string{
				"apiVersion": "step0-api",
				"kind":       "step0-kind",
				"name":       "step0",
				"namespace":  "bar",
			},
			"uri": "/extra/path",
		}, {
			"ref": map[string]string{
				"apiVersion": "step1-api",
				"kind":       "step1-kind",
				"name":       "step1",
				"namespace":  "bar",
			},
			"uri": "/extra/path",
		}},
		"reply": map[string]interface{}{
			"ref": map[string]string{
				"apiVersion": "reply-api",
				"kind":       "reply-kind",
				"name":       "reply",
				"namespace":  "bar",
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
	// apiVersion: flows.knative.dev/v1
	// kind: Sequence
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   channelTemplate:
	//     apiVersion: channelimpl/v1
	//     kind: mychannel
	//     spec:
	//       delivery:
	//         retry: 8
	//       thing2: value2
	//   steps:
	//     -
	//       ref:
	//         kind: step0-kind
	//         namespace: bar
	//         name: step0
	//         apiVersion: step0-api
	//       uri: /extra/path
	//     -
	//       ref:
	//         kind: step1-kind
	//         namespace: bar
	//         name: step1
	//         apiVersion: step1-api
	//       uri: /extra/path
	//   reply:
	//     ref:
	//       kind: reply-kind
	//       namespace: bar
	//       name: reply
	//       apiVersion: reply-api
	//     uri: /extra/path
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"channelTemplate": map[string]interface{}{
			"apiVersion": "channelimpl/v1",
			"kind":       "mychannel",
			"spec": map[string]string{
				"thing1": "value1",
				"thing2": "value2",
			},
		},
		"steps": []map[string]interface{}{{
			"ref": map[string]string{
				"apiVersion": "step0-api",
				"kind":       "step0-kind",
				"name":       "step0",
				"namespace":  "bar",
			},
			"uri": "/extra/path",
		}, {
			"ref": map[string]string{
				"apiVersion": "step1-api",
				"kind":       "step1-kind",
				"name":       "step1",
				"namespace":  "bar",
			},
			"uri": "/extra/path",
		}},
		"reply": map[string]interface{}{
			"ref": map[string]string{
				"apiVersion": "reply-api",
				"kind":       "reply-kind",
				"name":       "reply",
				"namespace":  "bar",
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
	// apiVersion: flows.knative.dev/v1
	// kind: Sequence
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   channelTemplate:
	//     apiVersion: channelimpl/v1
	//     kind: mychannel
	//     spec:
	//       thing1: value1
	//       thing2: value2
	//   steps:
	//     -
	//       ref:
	//         kind: step0-kind
	//         namespace: bar
	//         name: step0
	//         apiVersion: step0-api
	//       uri: /extra/path
	//     -
	//       ref:
	//         kind: step1-kind
	//         namespace: bar
	//         name: step1
	//         apiVersion: step1-api
	//       uri: /extra/path
	//   reply:
	//     ref:
	//       kind: reply-kind
	//       namespace: bar
	//       name: reply
	//       apiVersion: reply-api
	//     uri: /extra/path
}

func Example_withStep() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	sequence.WithStep(&v1.KReference{
		Kind:       "step0-kind",
		APIVersion: "step0-api",
		Namespace:  "bar",
		Name:       "step0name",
	}, "/extra/path")(cfg)

	sequence.WithStep(&v1.KReference{
		Kind:       "step1-kind",
		APIVersion: "step1-api",
		Namespace:  "bar",
		Name:       "step1name",
	}, "")(cfg)

	sequence.WithStep(nil, "http://full/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Sequence
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   steps:
	//     -
	//       ref:
	//         kind: step0-kind
	//         namespace: bar
	//         name: step0name
	//         apiVersion: step0-api
	//       uri: /extra/path
	//     -
	//       ref:
	//         kind: step1-kind
	//         namespace: bar
	//         name: step1name
	//         apiVersion: step1-api
	//     -
	//       uri: http://full/path
}

func Example_withReply() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	sequence.WithReply(&v1.KReference{
		Kind:       "repkind",
		APIVersion: "repversion",
		Name:       "repname",
	}, "/extra/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Sequence
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   steps:
	//   reply:
	//     ref:
	//       kind: repkind
	//       namespace: bar
	//       name: repname
	//       apiVersion: repversion
	//     uri: /extra/path
}
