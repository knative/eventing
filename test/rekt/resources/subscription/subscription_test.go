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

package subscription_test

import (
	"os"

	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/subscription"
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
	// apiVersion: messaging.knative.dev/v1
	// kind: Subscription
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
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
	// apiVersion: messaging.knative.dev/v1
	// kind: Subscription
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
		"subscriber": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "subkind",
				"name":       "subname",
				"apiVersion": "subversion",
			},
			"uri": "/extra/path",
		},
		"reply": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "repkind",
				"name":       "repname",
				"apiVersion": "repversion",
			},
			"uri": "/extra/path",
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
	// kind: Subscription
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
	//   reply:
	//     ref:
	//       kind: repkind
	//       namespace: bar
	//       name: repname
	//       apiVersion: repversion
	//     uri: /extra/path
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

func ExampleWithChannel() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	subscription.WithChannel(&v1.KReference{
		Kind:       "chkind",
		Name:       "chname",
		APIVersion: "chversion",
	})(cfg)
	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: messaging.knative.dev/v1
	// kind: Subscription
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   channel:
	//     kind: chkind
	//     name: chname
	//     apiVersion: chversion
}

func ExampleWithSubscriber() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	subscription.WithSubscriber(&v1.KReference{
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
	// apiVersion: messaging.knative.dev/v1
	// kind: Subscription
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithReply() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	subscription.WithReply(&v1.KReference{
		Kind:       "repkind",
		Name:       "repname",
		APIVersion: "repversion",
	}, "/extra/path")(cfg)

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: messaging.knative.dev/v1
	// kind: Subscription
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   reply:
	//     ref:
	//       kind: repkind
	//       namespace: bar
	//       name: repname
	//       apiVersion: repversion
	//     uri: /extra/path
}
