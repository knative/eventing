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

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var templates embed.FS

// Install creates the necessary ServiceAccount, Role, RoleBinding for the eventshub.
// The resources are named according to the current namespace defined in the environment.
func Install(cfg map[string]interface{}) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, templates, cfg); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatal(err)
		}
	}
}
