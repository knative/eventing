/*
Copyright 2024 The Knative Authors

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

package eventpolicy_test

import (
	"embed"
	"knative.dev/eventing/test/rekt/resources/broker"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/rekt/resources/eventpolicy"
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
		"name":      "foo",
		"namespace": "bar",
		"from": []map[string]interface{}{
			{
				"ref": map[string]string{
					"kind":       "Broker",
					"name":       "my-broker",
					"namespace":  "my-ns",
					"apiVersion": "eventing.knative.dev/v1",
				},
			},
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1alpha1
	// kind: EventPolicy
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   from:
	//     - ref:
	//         apiVersion: eventing.knative.dev/v1
	//         kind: Broker
	//         name: my-broker
	//         namespace: my-ns
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	cfgFn := []manifest.CfgFn{
		eventpolicy.WithToRef(
			broker.GVR().GroupVersion().WithKind("Broker"),
			"my-broker"),
		eventpolicy.WithToSelector(
			broker.GVR().GroupVersion().WithKind("Broker"),
			&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"matchlabel1": "matchlabelvalue1",
					"matchlabel2": "matchlabelvalue2",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "matchlabelselector1",
						Values:   []string{"matchlabelselectorvalue1"},
						Operator: metav1.LabelSelectorOpIn,
					},
				},
			}),
		eventpolicy.WithFromRef(
			broker.GVR().GroupVersion().WithKind("Broker"),
			"my-broker",
			"my-ns-2",
		),
		eventpolicy.WithFromSubject("my-sub"),
	}

	for _, fn := range cfgFn {
		fn(cfg)
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1alpha1
	// kind: EventPolicy
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   to:
	//     - ref:
	//         apiVersion: eventing.knative.dev/v1
	//         kind: Broker
	//         name: my-broker
	//     - selector:
	//         apiVersion: eventing.knative.dev/v1
	//         kind: Broker
	//         matchLabels:
	//           matchlabel1: matchlabelvalue1
	//           matchlabel2: matchlabelvalue2
	//         matchExpressions:
	//           - key: matchlabelselector1
	//             operator: In
	//             values:
	//             - matchlabelselectorvalue1
	//   from:
	//     - ref:
	//         apiVersion: eventing.knative.dev/v1
	//         kind: Broker
	//         name: my-broker
	//         namespace: my-ns-2
	//     - sub: my-sub
}
