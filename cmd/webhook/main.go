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

	"github.com/knative/eventing/pkg/channeldefaulter"
	"github.com/knative/eventing/pkg/defaultchannel"

	"go.uber.org/zap"

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/logconfig"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

func main() {
	flag.Parse()
	// Read the logging config and setup a logger.
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	config, err := logging.NewConfigFromMap(cm)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.WebhookName())
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.WebhookName()))

	logger.Infow("Starting the Eventing Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalw("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatalw("Failed to get the client set", zap.Error(err))
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	configMapWatcher.Watch(logconfig.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.WebhookName()))

	// Watch the default-channel-webhook ConfigMap and dynamically update the default
	// ClusterChannelProvisioner.
	channelDefaulter := channeldefaulter.New(logger.Desugar())
	eventingv1alpha1.ChannelDefaulterSingleton = channelDefaulter
	configMapWatcher.Watch(channeldefaulter.ConfigMapName, channelDefaulter.UpdateConfigMap)

	// Watch the default-ch-webhook ConfigMap and dynamically update the default
	// Channel CRD.
	chDefaulter := defaultchannel.New(logger.Desugar())
	eventingduckv1alpha1.ChannelDefaulterSingleton = chDefaulter
	configMapWatcher.Watch(defaultchannel.ConfigMapName, chDefaulter.UpdateConfigMap)

	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start webhook configmap watcher: %v", err)
	}

	options := webhook.ControllerOptions{
		ServiceName:    logconfig.WebhookName(),
		DeploymentName: logconfig.WebhookName(),
		Namespace:      system.Namespace(),
		Port:           8443,
		SecretName:     "eventing-webhook-certs",
		WebhookName:    "webhook.eventing.knative.dev",
	}
	controller := webhook.AdmissionController{
		Client:  kubeClient,
		Options: options,
		Handlers: map[schema.GroupVersionKind]webhook.GenericCRD{
			// For group eventing.knative.dev,
			eventingv1alpha1.SchemeGroupVersion.WithKind("Broker"):                    &eventingv1alpha1.Broker{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("Channel"):                   &eventingv1alpha1.Channel{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("ClusterChannelProvisioner"): &eventingv1alpha1.ClusterChannelProvisioner{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("Subscription"):              &eventingv1alpha1.Subscription{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"):                   &eventingv1alpha1.Trigger{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("EventType"):                 &eventingv1alpha1.EventType{},
			// For group messaging.knative.dev.
			messagingv1alpha1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1alpha1.InMemoryChannel{},
			messagingv1alpha1.SchemeGroupVersion.WithKind("Sequence"):        &messagingv1alpha1.Sequence{},
			messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1alpha1.Channel{},
		},
		Logger: logger,
	}
	if err != nil {
		logger.Fatalw("Failed to create the admission controller", zap.Error(err))
	}
	if err = controller.Run(stopCh); err != nil {
		logger.Errorw("controller.Run() failed", zap.Error(err))
	}
	logger.Infow("Webhook stopping")
}
