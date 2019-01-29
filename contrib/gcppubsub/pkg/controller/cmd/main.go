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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/controller/channel"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/controller/clusterchannelprovisioner"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// These are Environment variable names.
	defaultGcpProjectEnv      = "DEFAULT_GCP_PROJECT"
	defaultSecretNamespaceEnv = "DEFAULT_SECRET_NAMESPACE"
	defaultSecretNameEnv      = "DEFAULT_SECRET_NAME"
	defaultSecretKeyEnv       = "DEFAULT_SECRET_KEY"
)

// This is the main method for the GCP PubSub Channel controller. It reconciles the
// ClusterChannelProvisioner itself and Channels that use the 'gcp-pubsub' provisioner. It does not
// handle the anything at the data layer.
func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig)
	defer logger.Sync()
	logger = logger.With(
		zap.String("eventing.knative.dev/clusterChannelProvisioner", clusterchannelprovisioner.Name),
		zap.String("eventing.knative.dev/clusterChannelProvisionerComponent", "Controller"),
	)
	flag.Parse()

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	eventingv1alpha1.AddToScheme(mgr.GetScheme())
	istiov1alpha3.AddToScheme(mgr.GetScheme())

	// The controllers for both the ClusterChannelProvisioner and the Channels created by that
	// ClusterChannelProvisioner run in this process.
	_, err = clusterchannelprovisioner.ProvideController(mgr, logger.Desugar())
	if err != nil {
		logger.Fatal("Unable to create Provisioner controller", zap.Error(err))
	}

	defaultGcpProject := getRequiredEnv(defaultGcpProjectEnv)
	defaultSecret := v1.ObjectReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Namespace:  getRequiredEnv(defaultSecretNamespaceEnv),
		Name:       getRequiredEnv(defaultSecretNameEnv),
	}
	defaultSecretKey := getRequiredEnv(defaultSecretKeyEnv)
	_, err = channel.ProvideController(defaultGcpProject, &defaultSecret, defaultSecretKey)(mgr, logger.Desugar())
	if err != nil {
		logger.Fatal("Unable to create Channel controller", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
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
