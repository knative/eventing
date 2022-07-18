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

package pod_test

import (
	"embed"
	"os"

	"knative.dev/eventing/test/rekt/resources/pod"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Example_min() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"image":     "baz",
		"port":      "8080",
		"labels":    map[string]string{"app": "bla"},
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
	//     app: bla
	// spec:
	//   containers:
	//     - name: user-container
	//       image: baz
	//       ports:
	//         - containerPort: 8080
}

func Example_withLabels() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"image":     "baz",
		"port":      "8080",
		"labels":    map[string]string{"app": "bla"},
	}

	pod.WithLabels(map[string]string{"overwrite": "yes"})(cfg)

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
	//     overwrite: yes
	// spec:
	//   containers:
	//     - name: user-container
	//       image: baz
	//       ports:
	//         - containerPort: 8080
}

func Example_withImage() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"image":     "baz",
		"port":      "8080",
		"labels":    map[string]string{"app": "bla"},
	}

	pod.WithImage("myimage")(cfg)

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
	//     app: bla
	// spec:
	//   containers:
	//     - name: user-container
	//       image: myimage
	//       ports:
	//         - containerPort: 8080
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"image":     "baz",
		"port":      "8080",
		"labels":    map[string]string{"app": "bla"},
	}

	pod.WithLabels(map[string]string{"overwrite": "yes"})(cfg)
	pod.WithImage("myimage")(cfg)

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
	//     overwrite: yes
	// spec:
	//   containers:
	//     - name: user-container
	//       image: myimage
	//       ports:
	//         - containerPort: 8080
}
