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

package parallel_test

import (
	"embed"
	"os"

	v1 "knative.dev/pkg/apis/duck/v1"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/parallel"
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
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
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
		"branches": []map[string]interface{}{{
			"filter": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "filter1-api",
					"kind":       "filter1-kind",
					"name":       "filter1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"subscriber": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "subscriber1-api",
					"kind":       "subscriber1-kind",
					"name":       "subscriber1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"reply": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "reply1-api",
					"kind":       "reply1-kind",
					"name":       "reply1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			}}, {
			"filter": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "filter2-api",
					"kind":       "filter2-kind",
					"name":       "filter2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"subscriber": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "subscriber2-api",
					"kind":       "subscriber2-kind",
					"name":       "subscriber2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"reply": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "reply2-api",
					"kind":       "reply2-kind",
					"name":       "reply2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			}},
		},
		"reply": map[string]interface{}{
			"ref": map[string]string{
				"apiVersion": "reply1-api",
				"kind":       "reply1-kind",
				"name":       "reply1",
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
	// kind: Parallel
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
	//   branches:
	//     -
	//       filter:
	//         ref:
	//           kind: filter1-kind
	//           namespace: bar
	//           name: filter1
	//           apiVersion: filter1-api
	//         uri: /extra/path
	//       subscriber:
	//         ref:
	//           kind: subscriber1-kind
	//           namespace: bar
	//           name: subscriber1
	//           apiVersion: subscriber1-api
	//         uri: /extra/path
	//       reply:
	//         ref:
	//           kind: reply1-kind
	//           namespace: bar
	//           name: reply1
	//           apiVersion: reply1-api
	//         uri: /extra/path
	//     -
	//       filter:
	//         ref:
	//           kind: filter2-kind
	//           namespace: bar
	//           name: filter2
	//           apiVersion: filter2-api
	//         uri: /extra/path
	//       subscriber:
	//         ref:
	//           kind: subscriber2-kind
	//           namespace: bar
	//           name: subscriber2
	//           apiVersion: subscriber2-api
	//         uri: /extra/path
	//       reply:
	//         ref:
	//           kind: reply2-kind
	//           namespace: bar
	//           name: reply2
	//           apiVersion: reply2-api
	//         uri: /extra/path
	//   reply:
	//     ref:
	//       kind: reply1-kind
	//       namespace: bar
	//       name: reply1
	//       apiVersion: reply1-api
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
		"branches": []map[string]interface{}{{
			"filter": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "filter1-api",
					"kind":       "filter1-kind",
					"name":       "filter1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"subscriber": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "subscriber1-api",
					"kind":       "subscriber1-kind",
					"name":       "subscriber1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"reply": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "reply1-api",
					"kind":       "reply1-kind",
					"name":       "reply1",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			}}, {
			"filter": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "filter2-api",
					"kind":       "filter2-kind",
					"name":       "filter2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"subscriber": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "subscriber2-api",
					"kind":       "subscriber2-kind",
					"name":       "subscriber2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			},
			"reply": map[string]interface{}{
				"ref": map[string]string{
					"apiVersion": "reply2-api",
					"kind":       "reply2-kind",
					"name":       "reply2",
					"namespace":  "bar",
				},
				"uri": "/extra/path",
			}},
		},
		"reply": map[string]interface{}{
			"ref": map[string]string{
				"apiVersion": "reply1-api",
				"kind":       "reply1-kind",
				"name":       "reply1",
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
	// kind: Parallel
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
	//   branches:
	//     -
	//       filter:
	//         ref:
	//           kind: filter1-kind
	//           namespace: bar
	//           name: filter1
	//           apiVersion: filter1-api
	//         uri: /extra/path
	//       subscriber:
	//         ref:
	//           kind: subscriber1-kind
	//           namespace: bar
	//           name: subscriber1
	//           apiVersion: subscriber1-api
	//         uri: /extra/path
	//       reply:
	//         ref:
	//           kind: reply1-kind
	//           namespace: bar
	//           name: reply1
	//           apiVersion: reply1-api
	//         uri: /extra/path
	//     -
	//       filter:
	//         ref:
	//           kind: filter2-kind
	//           namespace: bar
	//           name: filter2
	//           apiVersion: filter2-api
	//         uri: /extra/path
	//       subscriber:
	//         ref:
	//           kind: subscriber2-kind
	//           namespace: bar
	//           name: subscriber2
	//           apiVersion: subscriber2-api
	//         uri: /extra/path
	//       reply:
	//         ref:
	//           kind: reply2-kind
	//           namespace: bar
	//           name: reply2
	//           apiVersion: reply2-api
	//         uri: /extra/path
	//   reply:
	//     ref:
	//       kind: reply1-kind
	//       namespace: bar
	//       name: reply1
	//       apiVersion: reply1-api
	//     uri: /extra/path
}

func Example_withSubscriberAt() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	parallel.WithSubscriberAt(0, &v1.KReference{
		Kind:       "sub0kind",
		APIVersion: "sub0version",
		Name:       "sub0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	parallel.WithSubscriberAt(1, &v1.KReference{
		Kind:       "sub1kind",
		APIVersion: "sub1version",
		Name:       "sub1name",
		Namespace:  "bar",
	}, "")(cfg)

	parallel.WithSubscriberAt(2, nil, "http://full/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
	//     -
	//       subscriber:
	//         ref:
	//           kind: sub0kind
	//           namespace: bar
	//           name: sub0name
	//           apiVersion: sub0version
	//         uri: /extra/path
	//     -
	//       subscriber:
	//         ref:
	//           kind: sub1kind
	//           namespace: bar
	//           name: sub1name
	//           apiVersion: sub1version
	//     -
	//       subscriber:
	//         uri: http://full/path
}

func Example_withFilterAt() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	parallel.WithFilterAt(0, &v1.KReference{
		Kind:       "fil0kind",
		APIVersion: "fil0version",
		Name:       "fil0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	parallel.WithFilterAt(1, &v1.KReference{
		Kind:       "fil1kind",
		APIVersion: "fil1version",
		Name:       "fil1name",
		Namespace:  "bar",
	}, "")(cfg)

	parallel.WithFilterAt(2, nil, "http://full/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
	//     -
	//       filter:
	//         ref:
	//           kind: fil0kind
	//           namespace: bar
	//           name: fil0name
	//           apiVersion: fil0version
	//         uri: /extra/path
	//     -
	//       filter:
	//         ref:
	//           kind: fil1kind
	//           namespace: bar
	//           name: fil1name
	//           apiVersion: fil1version
	//     -
	//       filter:
	//         uri: http://full/path
}

func Example_withReplyAt() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	parallel.WithReplyAt(0, &v1.KReference{
		Kind:       "rep0kind",
		APIVersion: "rep0version",
		Name:       "rep0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	parallel.WithReplyAt(1, &v1.KReference{
		Kind:       "rep1kind",
		APIVersion: "rep1version",
		Name:       "rep1name",
		Namespace:  "bar",
	}, "")(cfg)

	parallel.WithReplyAt(2, nil, "http://full/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
	//     -
	//       reply:
	//         ref:
	//           kind: rep0kind
	//           namespace: bar
	//           name: rep0name
	//           apiVersion: rep0version
	//         uri: /extra/path
	//     -
	//       reply:
	//         ref:
	//           kind: rep1kind
	//           namespace: bar
	//           name: rep1name
	//           apiVersion: rep1version
	//     -
	//       reply:
	//         uri: http://full/path
}

func Example_withFullAt() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	parallel.WithSubscriberAt(0, &v1.KReference{
		Kind:       "sub0kind",
		APIVersion: "sub0version",
		Name:       "sub0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	parallel.WithReplyAt(0, &v1.KReference{
		Kind:       "rep0kind",
		APIVersion: "rep0version",
		Name:       "rep0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	parallel.WithFilterAt(0, &v1.KReference{
		Kind:       "fil0kind",
		APIVersion: "fil0version",
		Name:       "fil0name",
		Namespace:  "bar",
	}, "/extra/path")(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: flows.knative.dev/v1
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
	//     -
	//       filter:
	//         ref:
	//           kind: fil0kind
	//           namespace: bar
	//           name: fil0name
	//           apiVersion: fil0version
	//         uri: /extra/path
	//       subscriber:
	//         ref:
	//           kind: sub0kind
	//           namespace: bar
	//           name: sub0name
	//           apiVersion: sub0version
	//         uri: /extra/path
	//       reply:
	//         ref:
	//           kind: rep0kind
	//           namespace: bar
	//           name: rep0name
	//           apiVersion: rep0version
	//         uri: /extra/path
}

func Example_withReply() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	parallel.WithReply(&v1.KReference{
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
	// kind: Parallel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   branches:
	//   reply:
	//     ref:
	//       kind: repkind
	//       namespace: bar
	//       name: repname
	//       apiVersion: repversion
	//     uri: /extra/path
}
