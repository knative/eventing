/*
Copyright 2022 The Knative Authors

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

package job_test

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
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"image":     "gcr.io/knative-samples/helloworld-go",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)

	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: batch/v1
	// kind: CronJob
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   schedule: "* * * * *"
	//   jobTemplate:
	//     metadata:
	//       labels:
	//         app: foo
	//     spec:
	//       template:
	//         spec:
	//           containers:
	//           - name: user-container
	//             image: gcr.io/knative-samples/helloworld-go
	//             imagePullPolicy: IfNotPresent
	//             env:
	//             - name: ONE_SHOT
	//               value: "true"
	//             - name: POD_NAME
	//               value: heartbeats
	//             - name: POD_NAMESPACE
	//               value: bar
	//           restartPolicy: Never
}
