/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"log"
	"os"

	"github.com/knative/eventing/pkg/provisioners"
	v1 "k8s.io/api/core/v1"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/dispatcher/dispatcher"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/dispatcher/receiver"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/controller/clusterchannelprovisioner"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	defaultGcpProjectEnv      = "DEFAULT_GCP_PROJECT"
	defaultSecretNamespaceEnv = "DEFAULT_SECRET_NAMESPACE"
	defaultSecretNameEnv      = "DEFAULT_SECRET_NAME"
	defaultSecretKeyEnv       = "DEFAULT_SECRET_KEY"
)

// This is the main method for the GCP PubSub Channel dispatcher. It handles all the data-plane
// activity for GCP PubSub Channels. It receives all events being sent to any gcp-pubsub Channel
// (via the receiver below) and watches all GCP PubSub Subscriptions (via the dispatcher below),
// sending events out when any are available.
func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig)
	defer logger.Sync()
	logger = logger.With(
		zap.String("eventing.knative.dev/clusterChannelProvisioner", clusterchannelprovisioner.Name),
		zap.String("eventing.knative.dev/clusterChannelProvisionerComponent", "Dispatcher"),
	)
	flag.Parse()

	logger.Info("Starting...")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	eventingv1alpha1.AddToScheme(mgr.GetScheme())

	defaultGcpProject := getRequiredEnv(defaultGcpProjectEnv)
	defaultSecret := v1.ObjectReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Namespace:  getRequiredEnv(defaultSecretNamespaceEnv),
		Name:       getRequiredEnv(defaultSecretNameEnv),
	}
	defaultSecretKey := getRequiredEnv(defaultSecretKeyEnv)

	// We are running both the receiver (takes messages in from the cluster and writes them to
	// PubSub) and the dispatcher (takes messages in PubSub and sends them in cluster) in this
	// binary.

	_, mr := receiver.New(logger.Desugar(), mgr.GetClient(), util.GcpPubSubClientCreator, defaultGcpProject, &defaultSecret, defaultSecretKey)
	err = mgr.Add(mr)
	if err != nil {
		logger.Fatal("Unable to add the MessageReceiver to the manager", zap.Error(err))
	}

	// TODO Move this to just before mgr.Start(). We need to pass the stopCh to dispatcher.New
	// because of https://github.com/kubernetes-sigs/controller-runtime/issues/103.

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	_, err = dispatcher.New(mgr, logger.Desugar(), defaultGcpProject, &defaultSecret, defaultSecretKey, stopCh)
	if err != nil {
		logger.Fatal("Unable to create the dispatcher", zap.Error(err))
	}

	// Start blocks forever.
	logger.Info("Manager starting...")
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}
