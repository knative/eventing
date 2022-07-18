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

package flaker_test

import (
	"embed"
	"os"

	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Example() {
	ctx := testlog.NewContext()
	images := map[string]string{
		"ko://knative.dev/eventing/test/test_images/event-flaker": "gcr.io/knative-samples/helloworld-go",
	}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"sink":      "uri://to/a/sink",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: Pod
	// metadata:
	//   name: foo
	//   namespace: bar
	//   labels:
	//     app: flaker-foo
	// spec:
	//   restartPolicy: "Never"
	//   containers:
	//     - name: flaker
	//       image: gcr.io/knative-samples/helloworld-go
	//       imagePullPolicy: "IfNotPresent"
	//       env:
	//         - name: "K_SINK"
	//           value: "uri://to/a/sink"
	// ---
	// apiVersion: v1
	// kind: Service
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   selector:
	//     app: flaker-foo
	//   ports:
	//     - protocol: TCP
	//       port: 80
	//       targetPort: 8080
}
