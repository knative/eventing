/*
Copyright 2020 The Knative Authors

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
	"strings"

	"knative.dev/reconciler-test/pkg/environment"
	eventshubrbac "knative.dev/reconciler-test/pkg/eventshub/rbac"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var templates embed.FS

func init() {
	environment.RegisterPackage(thisPackage)
}

// Install starts a new eventshub with the provided name
// Note: this function expects that the Environment is configured with the
// following options, otherwise it will panic:
//
//   ctx, env := global.Environment(
//     knative.WithKnativeNamespace("knative-namespace"),
//     knative.WithLoggingConfig,
//     knative.WithTracingConfig,
//     k8s.WithEventListener,
//   )
func Install(name string, options ...EventsHubOption) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()

		// Compute the user provided envs
		envs := make(map[string]string)
		if err := compose(options...)(ctx, envs); err != nil {
			t.Fatalf("Error while computing environment variables for eventshub: %s", err)
		}

		// eventshub needs tracing and logging config
		envs[ConfigLoggingEnv] = knative.LoggingConfigFromContext(ctx)
		envs[ConfigTracingEnv] = knative.TracingConfigFromContext(ctx)

		// Register the event info store to assert later the events published by the eventshub
		eventListener := k8s.EventListenerFromContext(ctx)
		registerEventsHubStore(eventListener, t, name, namespace)

		// Install ServiceAccount, Role, RoleBinding
		eventshubrbac.Install()(ctx, t)

		isReceiver := strings.Contains(envs["EVENT_GENERATORS"], "receiver")

		// Deploy
		if _, err := manifest.InstallYamlFS(ctx, templates, map[string]interface{}{
			"name":          name,
			"envs":          envs,
			"image":         ImageFromContext(ctx),
			"withReadiness": isReceiver,
		}); err != nil {
			t.Fatal(err)
		}

		k8s.WaitForPodRunningOrFail(ctx, t, name)
		k8s.WaitForReadyOrDoneOrFail(ctx, t, k8s.PodReference(namespace, name))

		// If the eventhubs starts an event receiver, we need to wait for the service endpoint to be synced
		if isReceiver {
			k8s.WaitForServiceEndpointsOrFail(ctx, t, name, 1)
			k8s.WaitForServiceReadyOrFail(ctx, t, name, "/health/ready")
		}
	}
}
