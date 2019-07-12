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

	"github.com/kelseyhightower/envconfig"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/controller/channel"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/controller/clusterchannelprovisioner"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// These are Environment variable names.
type envConfig struct {
	DefaultGcpProject      string `envconfig:"DEFAULT_GCP_PROJECT" required:"true"`
	DefaultSecretNamespace string `envconfig:"DEFAULT_SECRET_NAMESPACE" required:"true"`
	DefaultSecretName      string `envconfig:"DEFAULT_SECRET_NAME" required:"true"`
	DefaultSecretKey       string `envconfig:"DEFAULT_SECRET_KEY" required:"true"`
}

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

	// The controllers for both the ClusterChannelProvisioner and the Channels created by that
	// ClusterChannelProvisioner run in this process.
	_, err = clusterchannelprovisioner.ProvideController(mgr, logger.Desugar())
	if err != nil {
		logger.Fatal("Unable to create Provisioner controller", zap.Error(err))
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	defaultSecret := v1.ObjectReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Namespace:  env.DefaultSecretNamespace,
		Name:       env.DefaultSecretName,
	}
	_, err = channel.ProvideController(
		env.DefaultGcpProject, &defaultSecret, env.DefaultSecretKey)(mgr, logger.Desugar())
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
