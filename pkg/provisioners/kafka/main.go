package main

import (
	"flag"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	provisionerController "github.com/knative/eventing/pkg/provisioners/kafka/controller"
	"github.com/knative/eventing/pkg/provisioners/kafka/controller/channel"
)

var log = logf.Log.WithName("kafka-provisioner")

// SchemeFunc adds types to a Scheme.
type SchemeFunc func(*runtime.Scheme) error

// ProvideFunc adds a controller to a Manager.
type ProvideFunc func(mgr manager.Manager, log logr.Logger) (controller.Controller, error)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))
	entryLog := log.WithName("entrypoint")

	// Setup a Manager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to run controller manager")
		os.Exit(1)
	}

	// Add custom types to this array to get them into the manager's scheme.
	schemeFuncs := []SchemeFunc{
		v1alpha1.AddToScheme,
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

	for _, provider := range providers {
		if _, err := provider(mgr, log); err != nil {
			entryLog.Error(err, "unable to run controller manager")
			os.Exit(1)
		}
	}

	mgr.Start(signals.SetupSignalHandler())
}
