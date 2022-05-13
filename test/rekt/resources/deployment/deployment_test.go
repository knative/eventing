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

package deployment_test

import (
	"embed"
	"os"

	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Example() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"selectors": map[string]string{"app": "foo"},
		"image":     "gcr.io/knative-samples/helloworld-go",
		"port":      8080,
	}

	files, err := manifest.ExecuteYAML(yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: apps/v1
	// kind: Deployment
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   selector:
	//     matchLabels:
	//       app: foo
	//   template:
	//     metadata:
	//       labels:
	//         app: foo
	//     spec:
	//       containers:
	//         - name: user-container
	//           image: gcr.io/knative-samples/helloworld-go
	//           ports:
	//             - containerPort: 8080
}
