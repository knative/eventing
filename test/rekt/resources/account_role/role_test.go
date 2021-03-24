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

package account_role_test

import (
	"os"

	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/reconciler-test/pkg/manifest"
)

func Example() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":        "foo",
		"namespace":   "bar",
		"role":        "baz",
		"matchLabels": "whomp",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: ServiceAccount
	// metadata:
	//   name: foo
	//   namespace: bar
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRoleBinding
	// metadata:
	//   name: foo
	// subjects:
	//   - kind: ServiceAccount
	//     name: foo
	//     namespace: bar
	// roleRef:
	//   kind: ClusterRole
	//   name: baz
	//   apiGroup: rbac.authorization.k8s.io
}

func Example_matchLabel() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"role":       "baz",
		"matchLabel": "whomp",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: ServiceAccount
	// metadata:
	//   name: foo
	//   namespace: bar
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRole
	// metadata:
	//   name: baz
	// aggregationRule:
	//   clusterRoleSelectors:
	//     - matchLabels:
	//         whomp: "true"
	// rules: []
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRoleBinding
	// metadata:
	//   name: foo
	// subjects:
	//   - kind: ServiceAccount
	//     name: foo
	//     namespace: bar
	// roleRef:
	//   kind: ClusterRole
	//   name: baz
	//   apiGroup: rbac.authorization.k8s.io
}

func Example_channelableManipulator() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	account_role.AsChannelableManipulator(cfg)

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: ServiceAccount
	// metadata:
	//   name: foo
	//   namespace: bar
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRole
	// metadata:
	//   name: channelable-manipulator-collector-foo
	// aggregationRule:
	//   clusterRoleSelectors:
	//     - matchLabels:
	//         duck.knative.dev/channelable: "true"
	// rules: []
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRoleBinding
	// metadata:
	//   name: foo
	// subjects:
	//   - kind: ServiceAccount
	//     name: foo
	//     namespace: bar
	// roleRef:
	//   kind: ClusterRole
	//   name: channelable-manipulator-collector-foo
	//   apiGroup: rbac.authorization.k8s.io
}

func Example_addressableResolver() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	account_role.AsAddressableResolver(cfg)

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: ServiceAccount
	// metadata:
	//   name: foo
	//   namespace: bar
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRole
	// metadata:
	//   name: addressable-resolver-collector-foo
	// aggregationRule:
	//   clusterRoleSelectors:
	//     - matchLabels:
	//         duck.knative.dev/addressable: "true"
	// rules: []
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRoleBinding
	// metadata:
	//   name: foo
	// subjects:
	//   - kind: ServiceAccount
	//     name: foo
	//     namespace: bar
	// roleRef:
	//   kind: ClusterRole
	//   name: addressable-resolver-collector-foo
	//   apiGroup: rbac.authorization.k8s.io
}
