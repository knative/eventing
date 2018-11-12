package main

import (
	"flag"
	"os"

	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	eventingv1alpha "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	provisionerController "github.com/knative/eventing/pkg/provisioners/kafka/controller"
	"github.com/knative/eventing/pkg/provisioners/kafka/controller/channel"
)

// SchemeFunc adds types to a Scheme.
type SchemeFunc func(*runtime.Scheme) error

// ProvideFunc adds a controller to a Manager.
type ProvideFunc func(mgr manager.Manager, config *provisionerController.KafkaProvisionerConfig, logger *zap.Logger) (controller.Controller, error)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
	defer logger.Sync()

	// Setup a Manager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error(err, "unable to run controller manager")
		os.Exit(1)
	}

	// Add custom types to this array to get them into the manager's scheme.
	schemeFuncs := []SchemeFunc{
		eventingv1alpha.AddToScheme,
		istiov1alpha3.AddToScheme,
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
	provisionerConfig, err := provisionerController.GetProvisionerConfig("/etc/config-provisioner")

	if err != nil {
		logger.Error(err, "unable to run controller manager")
		os.Exit(1)
	}

	for _, provider := range providers {
		if _, err := provider(mgr, provisionerConfig, logger.Desugar()); err != nil {
			logger.Error(err, "unable to run controller manager")
			os.Exit(1)
		}
	}

	// Start blocks forever.
	err = mgr.Start(signals.SetupSignalHandler())
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}
