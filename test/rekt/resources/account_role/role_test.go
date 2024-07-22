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
	"embed"
	"os"

	rbacv1 "k8s.io/api/rbac/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Example() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":        "foo",
		"namespace":   "bar",
		"role":        "baz",
		"matchLabels": "whomp",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"role":       "baz",
		"matchLabel": "whomp",
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	account_role.AsChannelableManipulator(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	account_role.AsAddressableResolver(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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

func Example_withRoleAndRules() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	rule1 := rbacv1.PolicyRule{
		APIGroups: []string{"rule1ApiGroupA", "rule1ApiGroupB"},
		Resources: []string{"rule1ResourceA", "rule1ResourceB"},
		Verbs:     []string{"rule1VerbA", "rule1VerbB"},
	}

	rule2 := rbacv1.PolicyRule{
		APIGroups: []string{"rule2ApiGroupA", "rule2ApiGroupB"},
		Resources: []string{"rule2ResourceA", "rule2ResourceB"},
		Verbs:     []string{"rule2VerbA", "rule2VerbB"},
	}

	account_role.WithRole("baz")(cfg)
	account_role.WithRules(rule1, rule2)(cfg)

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
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
	// rules:
	//   - apiGroups:
	//       - rule1ApiGroupA
	//       - rule1ApiGroupB
	//     resources:
	//       - rule1ResourceA
	//       - rule1ResourceB
	//     verbs:
	//       - rule1VerbA
	//       - rule1VerbB
	//   - apiGroups:
	//       - rule2ApiGroupA
	//       - rule2ApiGroupB
	//     resources:
	//       - rule2ResourceA
	//       - rule2ResourceB
	//     verbs:
	//       - rule2VerbA
	//       - rule2VerbB
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
