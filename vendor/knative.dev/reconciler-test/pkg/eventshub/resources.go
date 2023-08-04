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

	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/environment"
	eventshubrbac "knative.dev/reconciler-test/pkg/eventshub/rbac"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/knativeservice"
	"knative.dev/reconciler-test/pkg/resources/service"
)

//go:embed 102-service.yaml 103-pod.yaml
var servicePodTemplates embed.FS

//go:embed 104-forwarder.yaml
var forwarderTemplates embed.FS

// Install starts a new eventshub with the provided name
// Note: this function expects that the Environment is configured with the
// following options, otherwise it will panic:
//
//	ctx, env := global.Environment(
//	  knative.WithKnativeNamespace("knative-namespace"),
//	  knative.WithLoggingConfig,
//	  knative.WithTracingConfig,
//	  k8s.WithEventListener,
//	)
func Install(name string, options ...EventsHubOption) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if err := registerImage(ctx); err != nil {
			t.Fatalf("Failed to install eventshub image: %v", err)
		}
		env := environment.FromContext(ctx)
		namespace := env.Namespace()
		log := logging.FromContext(ctx)

		// Compute the user provided envs
		envs := make(map[string]string)
		if err := compose(options...)(ctx, envs); err != nil {
			log.Fatalf("Error while computing environment variables for eventshub: %s", err)
		}

		// eventshub needs tracing and logging config
		envs[ConfigLoggingEnv] = knative.LoggingConfigFromContext(ctx)
		envs[ConfigTracingEnv] = knative.TracingConfigFromContext(ctx)

		// Register the event info store to assert later the events published by the eventshub
		eventListener := k8s.EventListenerFromContext(ctx)
		registerEventsHubStore(ctx, eventListener, name, namespace)

		isReceiver := strings.Contains(envs[EventGeneratorsEnv], "receiver")

		var withForwarder bool
		// Allow forwarder only when eventshub is receiver.
		if isForwarder(ctx) && isReceiver {
			withForwarder = isForwarder(ctx)
		}

		serviceName := name
		// When forwarder is included we need to rename the eventshub service to
		// prevent conflict with the forwarder service.
		if withForwarder {
			serviceName = feature.MakeRandomK8sName(name)
		}

		cfg := map[string]interface{}{
			"name":          name,
			"serviceName":   serviceName,
			"envs":          envs,
			"image":         ImageFromContext(ctx),
			"withReadiness": isReceiver,
		}

		// Install ServiceAccount, Role, RoleBinding
		eventshubrbac.Install(cfg)(ctx, t)

		if ic := environment.GetIstioConfig(ctx); ic.Enabled {
			manifest.WithIstioPodAnnotations(cfg)
		}

		manifest.PodSecurityCfgFn(ctx, t)(cfg)

		// Deploy Service/Pod
		if _, err := manifest.InstallYamlFS(ctx, servicePodTemplates, cfg); err != nil {
			log.Fatal(err)
		}

		k8s.WaitForPodReadyOrSucceededOrFail(ctx, t, name)

		// If the eventhubs starts an event receiver, we need to wait for the service endpoint to be synced
		if isReceiver {
			k8s.WaitForServiceEndpointsOrFail(ctx, t, serviceName, 1)
			k8s.WaitForServiceReadyOrFail(ctx, t, serviceName, "/health/ready")
		}

		if withForwarder {
			sinkURL, err := service.Address(ctx, serviceName)
			if err != nil {
				log.Fatal(err)
			}
			// At this point env contains "receiver" so we need to override it.
			envs[EventGeneratorsEnv] = "forwarder"
			// No event recording desired, just logging.
			envs[EventLogsEnv] = "logger"
			cfg["envs"] = envs
			cfg["sink"] = sinkURL

			// Deploy Forwarder
			if _, err := manifest.InstallYamlFS(ctx, forwarderTemplates, cfg); err != nil {
				log.Fatal(err)
			}
			knativeservice.IsReady(name)
		}
	}
}
