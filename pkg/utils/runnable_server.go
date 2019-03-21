/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RunnableServer is a small wrapper around http.Server so that it matches the
// manager.Runnable interface.
type RunnableServer struct {
	// Server is the http.Server to wrap.
	*http.Server

	// ShutdownTimeout is the duration to wait for the http.Server to gracefully
	// shut down when the stop channel is closed. If this is zero or negative,
	// the http.Server will be immediately closed instead.
	ShutdownTimeout time.Duration

	// Logger is used by RunnableServer to log lifecycle and error messages.
	Logger *zap.Logger

	// WaitGroup is a temporary workaround for Manager returning immediately
	// without waiting for Runnables to stop. See
	// https://github.com/kubernetes-sigs/controller-runtime/issues/350.
	WaitGroup *sync.WaitGroup
}

// Start the server. The server will be shut down when StopCh is closed.
func (r *RunnableServer) Start(stopCh <-chan struct{}) error {
	logger := r.Logger.With(zap.String("address", r.Addr))
	logger.Info("Listening...")

	errCh := make(chan error)

	if r.WaitGroup != nil {
		r.WaitGroup.Add(1)
		defer r.WaitGroup.Done()
	}

	go func() {
		err := r.ListenAndServe()
		if err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var err error
	select {
	case err = <-errCh:
		logger.Error("Error running HTTP server", zap.Error(err))
	case <-stopCh:
		if r.ShutdownTimeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), r.ShutdownTimeout)
			defer cancel()
			logger.Info("Shutting down...")
			if err := r.Server.Shutdown(ctx); err != nil {
				logger.Error("Shutdown returned an error", zap.Error(err))
			} else {
				logger.Debug("Shutdown done")
			}
		} else {
			if err := r.Server.Close(); err != nil {
				logger.Error("Close returned an error", zap.Error(err))
			}
		}

	}
	return err
}
