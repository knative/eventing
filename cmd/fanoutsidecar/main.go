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
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/channelwatcher"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute

	port                int
	channelProvisioners listFlags
)

type listFlags []string

func (l *listFlags) String() string {
	return ""
}
func (l *listFlags) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func init() {
	flag.IntVar(&port, "sidecar_port", -1, "The port to run the sidecar on.")
	flag.Var(&channelProvisioners, "channel_provisioner", "The provisioner of the channels that will be watched.")
}

func main() {
	flag.Parse()

	lc := zap.NewProductionConfig()
	lc.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := lc.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	if port < 0 {
		logger.Fatal("--sidecar_port flag must be set")
	}

	if len(channelProvisioners) < 1 {
		logger.Fatal("--channel_provisioner must be specified")
	}

	sh, err := swappable.NewEmptyHandler(logger)
	if err != nil {
		logger.Fatal("Unable to create swappable.Handler", zap.Error(err))
	}

	mgr, err := setupChannelWatcher(logger, sh.UpdateConfig)
	if err != nil {
		logger.Fatal("Unable to create channel watcher.", zap.Error(err))
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
		s:      s,
	})
	if err != nil {
		logger.Fatal("Unable to add ListenAndServe", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	if err = mgr.Start(stopCh); err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err = s.Shutdown(ctx); err != nil {
		logger.Error("Shutdown returned an error", zap.Error(err))
	}
}

func setupChannelWatcher(logger *zap.Logger, configUpdated swappable.UpdateConfig) (manager.Manager, error) {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error("Error creating new maanger.", zap.Error(err))
		return nil, err
	}
	if err = v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Error("Error while adding eventing scheme to manager.", zap.Error(err))
		return nil, err
	}
	channelwatcher.New(mgr, logger, channelwatcher.UpdateConfigWatchHandler(configUpdated, shouldWatch))

	return mgr, nil
}

func shouldWatch(ch *v1alpha1.Channel) bool {
	if ch.Spec.Provisioner != nil && ch.Spec.Provisioner.Namespace == "" {
		for _, v := range channelProvisioners {
			if v == ch.Spec.Provisioner.Name {
				return true
			}
		}
	}
	return false
}

// runnableServer is a small wrapper around http.Server so that it matches the manager.Runnable
// interface.
type runnableServer struct {
	logger *zap.Logger
	s      *http.Server
}

func (r *runnableServer) Start(<-chan struct{}) error {
	r.logger.Info("Fanout sidecar listening", zap.String("address", r.s.Addr))
	return r.s.ListenAndServe()
}
