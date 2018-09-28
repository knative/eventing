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
	"flag"
	"fmt"
	"github.com/knative/eventing/pkg/sidecar/configmap/filesystem"
	"github.com/knative/eventing/pkg/sidecar/configmap/watcher"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"time"
)

var (
	readTimeout  = time.Minute
	writeTimeout = time.Minute
)

func main() {
	portFlag := flag.Int("sidecar_port", -1, "The port to run the sidecar on.")
	configMapFlag := flag.String("config_map_noticer", "", "The system to notice changes to the ConfigMap. Valid values are: 'volume', 'watcher'.")

	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	if *portFlag < 0 {
		logger.Fatal("--sidecar_port flag must be set")
	}

	sh, err := swappable.NewEmptyHandler(logger)
	if err != nil {
		logger.Fatal("Unable to create swappable.Handler", zap.Error(err))
	}

	// Setup something to notice that the ConfigMap has updated.
	switch *configMapFlag {
	case "volume":
		_, err = filesystem.NewConfigMapWatcher(logger, filesystem.ConfigDir, sh.UpdateConfig)
		if err != nil {
			logger.Fatal("Unable to create filesystem.configMapWatcher", zap.Error(err))
		}
	case "watcher":
		err = setupWatcher(logger, sh.UpdateConfig)
		if err != nil {
			logger.Fatal("Unable to create K8s ConfigMap watcher.", zap.Error(err))
		}
	default:
		logger.Fatal("Need to provide the --config_map_noticer flag")
	}

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", *portFlag),
		Handler:      sh,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	logger.Info("Fanout sidecar Listening...")
	s.ListenAndServe()
}

func setupWatcher(logger *zap.Logger, configUpdated swappable.UpdateConfig) error {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting manager.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	corev1.AddToScheme(mgr.GetScheme())


	_, err = watcher.NewWatcher(logger, mgr, configUpdated)
	if err != nil {
		logger.Fatal("Unable to create watcher.configMapWatcher", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	go mgr.Start(stopCh)
	return nil
}
