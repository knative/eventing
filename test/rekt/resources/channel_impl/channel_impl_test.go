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

package channel_impl_test

import (
	"embed"
	"log"
	"os"

	"github.com/kelseyhightower/envconfig"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
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
		"apiVersion": "example.com/v1",
		"kind":       "Sample",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: example.com/v1
	// kind: Sample
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_env() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	apiVersion, kind := channel_impl.GVK().ToAPIVersionAndKind()
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"kind":       kind,
		"apiVersion": apiVersion,
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: messaging.knative.dev/v1
	// kind: InMemoryChannel
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_setenv() {
	ctx := testlog.NewContext()
	images := map[string]string{}

	_ = os.Setenv("CHANNEL_GROUP_KIND", "Sample.example.com")
	_ = os.Setenv("CHANNEL_VERSION", "v2")

	if err := envconfig.Process("", &channel_impl.EnvCfg); err != nil {
		log.Fatal("Failed to process env var", err)
	}

	apiVersion, kind := channel_impl.GVK().ToAPIVersionAndKind()
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"kind":       kind,
		"apiVersion": apiVersion,
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: example.com/v2
	// kind: Sample
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"apiVersion": "example.com/v1",
		"kind":       "Sample",
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

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: example.com/v1
	// kind: Sample
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
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
