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

const (
	defaultConfigMapName = "natss-dispatcher-config-map"

	// The following are the only valid values of the config_map_noticer flag.
//	cmnfVolume  = "volume"
//	cmnfWatcher = "watcher"
)

var (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute

	port               int
	configMapNoticer   string
	configMapNamespace string
	configMapName      string
)
/*
func init() {
	flag.IntVar(&port, "sidecar_port", -1, "The port to run the sidecar on.")
	flag.StringVar(&configMapNoticer, "config_map_noticer", "", fmt.Sprintf("The system to notice changes to the ConfigMap. Valid values are: %s", configMapNoticerValues()))
	flag.StringVar(&configMapNamespace, "config_map_namespace", system.Namespace, "The namespace of the ConfigMap that is watched for configuration.")
	flag.StringVar(&configMapName, "config_map_name", defaultConfigMapName, "The name of the ConfigMap that is watched for configuration.")
}
*/
func main() {
//	flag.Parse()

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

	_, err = channel.ProvideController(mgr, logger)
	if err != nil {
		logger.Fatal("Unable to create Channel controller", zap.Error(err))
	}

/*
	sh, err := swappable.NewEmptyHandler(logger)  // TODO remove this...
	if err != nil {
		logger.Fatal("Unable to create swappable.Handler", zap.Error(err))
	}

	mgr1, err1 := setupConfigMapNoticer(logger, sh.UpdateConfig)
	if err1 != nil {
		logger.Fatal("Unable to create configMap noticer.", zap.Error(err1))
	}

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      sh,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
*/
	// Start both the manager (which notices ConfigMap changes) and the HTTP server.
	stopCh := signals.SetupSignalHandler()
	var g errgroup.Group
/*
	g.Go(func() error {
		// Start blocks forever, so run it in a goroutine.
		return mgr1.Start(stopCh)
	})
	logger.Info("Dispatcher sidecar Listening...", zap.String("Address", s.Addr))
	g.Go(s.ListenAndServe)
*/

	/*
	err = g.Wait()
	if err != nil {
		logger.Error("Either the HTTP server or the ConfigMap noticer failed.", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	s.Shutdown(ctx)
	*/

	// set up signals so we handle the first shutdown signal gracefully
	//stopCh := signals.SetupSignalHandler()

	logger.Info("Dispatcher starting...")
	dispatcher, err := dispatcher.NewDispatcher(logger)
	if err != nil {
		logger.Fatal("unable to create NATSS dispatcher.", zap.Error(err))
	}

	g.Go(func() error {
		// Setups message receiver and blocks
		return dispatcher.Start(stopCh)
	})


	logger.Info("Dispatcher controller starting...")
	// Start blocks forever.
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}
/*
func configMapNoticerValues() string {
	return strings.Join([]string{cmnfVolume, cmnfWatcher}, ", ")
}

func setupConfigMapNoticer(logger *zap.Logger, configUpdated swappable.UpdateConfig) (manager.Manager, error) {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		return nil, err
		logger.Error("Error starting manager.", zap.Error(err))
	}

	switch configMapNoticer {
	case cmnfVolume:
		err = setupConfigMapVolume(logger, mgr, configUpdated)
	case cmnfWatcher:
		err = setupConfigMapWatcher(logger, mgr, configUpdated)
	default:
		err = fmt.Errorf("need to provide the --config_map_noticer flag (valid values are %s)", configMapNoticerValues())
	}
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func setupConfigMapVolume(logger *zap.Logger, mgr manager.Manager, configUpdated swappable.UpdateConfig) error {
	cmn, err := filesystem.NewConfigMapWatcher(logger, filesystem.ConfigDir, configUpdated)
	if err != nil {
		logger.Error("Unable to create filesystem.ConifgMapWatcher", zap.Error(err))
		return err
	}
	mgr.Add(cmn)
	return nil
}

func setupConfigMapWatcher(logger *zap.Logger, mgr manager.Manager, configUpdated swappable.UpdateConfig) error {
	kc, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	cmw, err := watcher.NewWatcher(logger, kc, configMapNamespace, configMapName, configUpdated)
	if err != nil {
		return err
	}

	mgr.Add(cmw)
	return nil
}
*/