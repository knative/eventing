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

package eventshub

import (
	"context"
	"embed"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var templates embed.FS

// Install creates the necessary ServiceAccount, Role, RoleBinding for the eventshub.
func Install(cfg map[string]interface{}) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		WithPullSecrets(ctx, t)(cfg)
		if _, err := manifest.InstallYamlFS(ctx, templates, cfg); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatal(err)
		}
	}
}

func WithPullSecrets(ctx context.Context, t feature.T) manifest.CfgFn {
	namespace := environment.FromContext(ctx).Namespace()
	serviceAccount, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(namespace).Get(ctx, "default", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to read default SA in %s namespace: %v", namespace, err)
	}

	return func(cfg map[string]interface{}) {
		if len(serviceAccount.ImagePullSecrets) == 0 {
			return
		}
		if _, set := cfg["withPullSecrets"]; !set {
			cfg["withPullSecrets"] = map[string]interface{}{}
		}
		withPullSecrets := cfg["withPullSecrets"].(map[string]interface{})
		withPullSecrets["secrets"] = []string{}
		for _, secret := range serviceAccount.ImagePullSecrets {
			withPullSecrets["secrets"] = append(withPullSecrets["secrets"].([]string), secret.Name)
		}
	}
}
