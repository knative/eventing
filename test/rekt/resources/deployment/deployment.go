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

package deployment

import (
	"context"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/deployment"
)

// Install will create a Deployment with defaults that can be overwritten by
// the With* methods.
// Deprecated, use knative.dev/reconciler-test/pkg/resources/deployment.Install
func Install(name string) feature.StepFn {
	image := "ko://knative.dev/eventing/test/test_images/heartbeats"

	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)

		deployment.Install(name, image,
			deployment.WithSelectors(map[string]string{"app": name}),
			deployment.WithEnvs(map[string]string{
				"POD_NAME":      "heartbeats",
				"POD_NAMESPACE": env.Namespace(),
			}),
			deployment.WithPort(8080))(ctx, t)
	}
}

// AsRef returns a KRef for a Deployment without namespace.
// Deprecated, use knative.dev/reconciler-test/pkg/resources/deployment.AsRef
var AsRef = deployment.AsRef

// Deprecated, use knative.dev/reconciler-test/pkg/resources/deployment.AsTrackerReference
var AsTrackerReference = deployment.AsTrackerReference
