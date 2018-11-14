package main

import (
//	"flag"
//	"fmt"
	"log"
//	"net/http"
	"time"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/natss/dispatcher/channel"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
//	"github.com/knative/eventing/pkg/sidecar/swappable"
//	"github.com/knative/eventing/pkg/sidecar/configmap/filesystem"
//	"github.com/knative/eventing/pkg/sidecar/configmap/watcher"
//	"github.com/knative/eventing/pkg/system"
	"golang.org/x/sync/errgroup"
//	"k8s.io/client-go/kubernetes"
//	"strings"
	"github.com/knative/eventing/pkg/provisioners/natss/dispatcher/dispatcher"
)


var (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute

	port               int
	configMapNoticer   string
	configMapNamespace string
	configMapName      string
)

func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	eventingv1alpha1.AddToScheme(mgr.GetScheme())
	istiov1alpha3.AddToScheme(mgr.GetScheme())

	// Start both the manager (which notices ConfigMap changes) and the HTTP server.
	stopCh := signals.SetupSignalHandler()
	var g errgroup.Group

	logger.Info("Dispatcher starting...")
	dispatcher, err := dispatcher.NewDispatcher(logger)
	if err != nil {
		logger.Fatal("Unable to create NATSS dispatcher.", zap.Error(err))
	}

	g.Go(func() error {
		// Setups message receiver and blocks
		return dispatcher.Start(stopCh)
	})

	_, err = channel.ProvideController(dispatcher, mgr, logger)
	if err != nil {
		logger.Fatal("Unable to create Channel controller", zap.Error(err))
	}

	logger.Info("Dispatcher controller starting...")
	// Start blocks forever.
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}
