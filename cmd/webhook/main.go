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

	"go.uber.org/zap"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/webhook"

	v1alpha1channels "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/eventing/pkg/system"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	logLevelKey = "webhook"
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

	logger.Info("Starting the Configuration Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewDefaultWatcher(kubeClient, system.Namespace)

	configMapWatcher.Watch(logconfig.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Webhook, logconfig.Webhook))
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start webhook configmap watcher: %v", err)
	}

	options := webhook.ControllerOptions{
		ServiceName:    "webhook",
		DeploymentName: "webhook",
		Namespace:      system.Namespace,
		Port:           443,
		SecretName:     "webhook-certs",
		WebhookName:    "webhook.serving.knative.dev",
	}
	controller := webhook.AdmissionController{
		Client:  kubeClient,
		Options: options,
		Handlers: map[schema.GroupVersionKind]runtime.Object{
			v1alpha1channels.SchemeGroupVersion.WithKind("Bus"):        &v1alpha1channels.Bus{},
			v1alpha1channels.SchemeGroupVersion.WithKind("ClusterBus"): &v1alpha1channels.Bus{},
		},
		Logger: logger,
	}
	if err != nil {
		logger.Fatal("Failed to create the admission controller", zap.Error(err))
	}
	controller.Run(stopCh)
}

func main2() {
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
		logger.Fatal("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
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
		logger.Fatal("Failed to create the admission controller", zap.Error(err))
	}
	controller.Run(stopCh)
}
