/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"github.com/knative/eventing/contrib/kafka/pkg/utils"
	"os"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	provisionerController "github.com/knative/eventing/contrib/kafka/pkg/controller"
	"github.com/knative/eventing/contrib/kafka/pkg/controller/channel"
	eventingv1alpha "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// SchemeFunc adds types to a Scheme.
type SchemeFunc func(*runtime.Scheme) error

// ProvideFunc adds a controller to a Manager.
type ProvideFunc func(mgr manager.Manager, config *utils.KafkaConfig, logger *zap.Logger) (controller.Controller, error)

func main() {
	os.Exit(_main())
}

func _main() int {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
	defer logger.Sync()

	// Setup a Manager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error(err, "unable to run controller manager")
		return 1
	}

	// Add custom types to this array to get them into the manager's scheme.
	schemeFuncs := []SchemeFunc{
		eventingv1alpha.AddToScheme,
	}
	for _, schemeFunc := range schemeFuncs {
		schemeFunc(mgr.GetScheme())
	}

	// Add each controller's ProvideController func to this list to have the
	// manager run it.
	providers := []ProvideFunc{
		provisionerController.ProvideController,
		channel.ProvideController,
	}

	// TODO the underlying config map needs to be watched and the config should be reloaded if there is a change.
	provisionerConfig, err := utils.GetKafkaConfig("/etc/config-provisioner")

	if err != nil {
		logger.Error(err, "unable to run controller manager")
		return 1
	}

	for _, provider := range providers {
		if _, err := provider(mgr, provisionerConfig, logger.Desugar()); err != nil {
			logger.Error(err, "unable to run controller manager")
			return 1
		}
	}

	// Start blocks forever.
	err = mgr.Start(signals.SetupSignalHandler())
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
	return 0
}
