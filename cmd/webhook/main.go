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

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/eventing/pkg/signals"
	"github.com/knative/eventing/pkg/webhook"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	flag.Parse()
	defer glog.Flush()

	glog.Info("Starting the Configuration Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal("Failed to get in cluster config", err)
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		glog.Fatal("Failed to get the client set", err)
	}

	options := webhook.ControllerOptions{
		ServiceName:      "eventing-webhook",
		ServiceNamespace: system.Namespace,
		Port:             443,
		SecretName:       "eventing-webhook-certs",
		WebhookName:      "webhook.eventing.knative.dev",
	}
	controller, err := webhook.NewAdmissionController(clientset, options)
	if err != nil {
		glog.Fatal("Failed to create the admission controller", err)
	}
	controller.Run(stopCh)
}
g", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewDefaultWatcher(kubeClient, system.Namespace)
	configMapWatcher.Watch(logging.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logLevelKey))
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start configuration manager: %v", err)
	}

	options := webhook.ControllerOptions{
		ServiceName:    "eventing-webhook",
		DeploymentName: "eventing-webhook",
		Namespace:      system.Namespace,
		Port:           443,
		SecretName:     "eventing-webhook-certs",
		WebhookName:    "eventing-webhook.eventing.knative.dev",
	}
	controller := webhook.AdmissionController{
		Client:  kubeClient,
		Options: options,
		// TODO(mattmoor): Will we need to rework these to support versioning?
		GroupVersion: v1alpha1.SchemeGroupVersion,
		Handlers: map[string]runtime.Object{
			"Revision":      &v1alpha1.Revision{},
			"Configuration": &v1alpha1.Configuration{},
			"Route":         &v1alpha1.Route{},
			"Service":       &v1alpha1.Service{},
		},
		Logger: logger,
	}
	if err != nil {
		logger.Fatal("Failed to create the admission controller", zap.Error(err))
	}
	controller.Run(stopCh)
}
