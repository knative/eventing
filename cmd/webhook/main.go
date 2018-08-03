/*
Copyright 2018 The Knative Authors
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

package main

import (
	"flag"
	"log"

	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/eventing/pkg/signals"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/eventing/pkg/webhook"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/logging/logkey"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	flag.Parse()

	// Read the logging config and setup a logger.
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	config, err := logging.NewConfigFromMap(cm, logconfig.Webhook)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.Webhook)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.Webhook))

	logger.Info("Starting the Eventing Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to get in cluster config", err)
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", err)
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewDefaultWatcher(kubeClient, system.Namespace)
	configMapWatcher.Watch(logconfig.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Webhook, logconfig.Webhook))
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start webhook configmap watcher: %v", err)
	}

	// TODO(n3wscott): Send the logger to the controller.
	options := webhook.ControllerOptions{
		ServiceName:      "eventing-webhook",
		ServiceNamespace: system.Namespace,
		Port:             443,
		SecretName:       "eventing-webhook-certs",
		WebhookName:      "webhook.eventing.knative.dev",
	}
	controller, err := webhook.NewAdmissionController(kubeClient, options)
	if err != nil {
		logger.Fatal("Failed to create the admission controller", err)
	}
	controller.Run(stopCh)
}
