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

package eventtransform_test

import (
	"embed"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"os"

	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/eventtransform"
)

//go:embed *.yaml
var yaml embed.FS

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	eventtransform.WithAnnotations(map[string]interface{}{
		"annotation1": "value1",
	})(cfg)

	eventtransform.WithSpec(
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{
			Expression: `{"data": $ }`,
		}),
	)(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1alpha1
	// kind: EventTransform
	// metadata:
	//   name: foo
	//   namespace: bar
	//     annotation1: "value1"
	// spec:
	//   jsonata:
	//     expression: '{"data": $ }'
}
