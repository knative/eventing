/*
Copyright 2018 The Knative Authors

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

// A sidecar that implements filtering of Cloud Events sent out via HTTP. Implemented as an HTTP
// proxy that the main containers need to write through.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/knative/eventing/pkg/utils"

	"github.com/knative/eventing/pkg/sidecar/configmap/filesystem"
	"github.com/knative/eventing/pkg/sidecar/configmap/watcher"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

const (
	defaultConfigMapName = "in-memory-channel-dispatcher-config-map"

	// The following are the only valid values of the config_map_noticer flag.
	cmnfVolume  = "volume"
	cmnfWatcher = "watcher"
)

var (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute

	port               int
	configMapNoticer   string
	configMapNamespace string
	configMapName      string
)

func init() {
	flag.IntVar(&port, "sidecar_port", -1, "The port to run the sidecar on.")
	flag.StringVar(&configMapNoticer, "config_map_noticer", "", fmt.Sprintf("The system to notice changes to the ConfigMap. Valid values are: %s", configMapNoticerValues()))
	flag.StringVar(&configMapNamespace, "config_map_namespace", system.Namespace(), "The namespace of the ConfigMap that is watched for configuration.")
	flag.StringVar(&configMapName, "config_map_name", defaultConfigMapName, "The name of the ConfigMap that is watched for configuration.")
}

func configMapNoticerValues() string {
	return strings.Join([]string{cmnfVolume, cmnfWatcher}, ", ")
}

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	if port < 0 {
		logger.Fatal("--sidecar_port flag must be set")
	}

	sh, err := swappable.NewEmptyHandler(logger)
	if err != nil {
		logger.Fatal("Unable to create swappable.Handler", zap.Error(err))
	}

	mgr, err := setupConfigMapNoticer(logger, sh.UpdateConfig)
	if err != nil {
		logger.Fatal("Unable to create configMap noticer.", zap.Error(err))
	}

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      sh,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	err = mgr.Add(&runnableServer{
		logger: logger,
		s: s,
	})
	if err != nil {
		logger.Fatal("Unable to add ListenAndServe", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	err = s.Shutdown(ctx)
	if err != nil {
		logger.Error("Shutdown returned an error", zap.Error(err))
	}
}

func setupConfigMapNoticer(logger *zap.Logger, configUpdated swappable.UpdateConfig) (manager.Manager, error) {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error("Error starting manager.", zap.Error(err))
		return nil, err
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
	err = mgr.Add(cmn)
	if err != nil {
		logger.Error("Unable to add the config map watcher", zap.Error(err))
		return err
	}
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

	err = mgr.Add(utils.NewBlockingStart(logger, cmw))
	if err != nil {
		logger.Error("Unable to add the config map watcher", zap.Error(err))
		return err
	}
	return nil
}

// runnableServer is a small wrapper around http.Server so that it matches the manager.Runnable
// interface.
type runnableServer struct {
	logger *zap.Logger
	s *http.Server
}

func (r *runnableServer) Start(<-chan struct{}) error {
	r.logger.Info("Fanout sidecar listening", zap.String("address", r.s.Addr))
	return r.s.ListenAndServe()
}
