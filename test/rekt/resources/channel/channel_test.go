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

package channel_test

import (
	"embed"
	"encoding/json"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/reconciler-test/pkg/manifest"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/test/rekt/resources/channel"
)

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
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
	// apiVersion: messaging.knative.dev/v1
	// kind: Channel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_full() {
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

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: messaging.knative.dev/v1
	// kind: Channel
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

//go:embed *.yaml
var yaml embed.FS

func Example_withTemplate() {

	spec := map[string]string{
		"thing1": "value1",
		"thing2": "value2",
	}
	bytesSpec, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}

	re := &runtime.RawExtension{
		Raw: bytesSpec,
	}

	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}
	withTemplate := channel.WithTemplate(func(spec *messagingv1.ChannelTemplateSpec) error {
		spec.Spec = re
		return nil
	})
	withTemplate(cfg)

	files, err := manifest.ExecuteYAML(yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: messaging.knative.dev/v1
	// kind: Channel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   channelTemplate:
	//     apiVersion: messaging.knative.dev/v1
	//     kind: InMemoryChannel
	//     spec:
	//       thing1: value1
	//       thing2: value2
}
