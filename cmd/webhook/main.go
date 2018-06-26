/*
Copyright 2017 The Knative Authors
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
	"github.com/knative/eventing/pkg"
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
		ServiceNamespace: pkg.GetEventingSystemNamespace(),
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
