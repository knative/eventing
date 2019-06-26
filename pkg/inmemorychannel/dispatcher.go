/*
Copyright 2019 The Knative Authors

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
package inmemorychannel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	"github.com/knative/eventing/pkg/provisioners/swappable"
	pkgtracing "knative.dev/pkg/tracing"
	"go.uber.org/zap"
)

type Dispatcher interface {
	UpdateConfig(config *multichannelfanout.Config) error
}

type InMemoryDispatcher struct {
	handler *swappable.Handler
	server  *http.Server

	logger *zap.Logger
}

type InMemoryDispatcherArgs struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Handler      *swappable.Handler
	Logger       *zap.Logger
}

func (d *InMemoryDispatcher) UpdateConfig(config *multichannelfanout.Config) error {
	return d.handler.UpdateConfig(config)
}

// Start starts the inmemory dispatcher's message processing.
// This is a blocking call.
func (d *InMemoryDispatcher) Start(stopCh <-chan struct{}) error {
	d.logger.Info("in memory dispatcher listening", zap.String("address", d.server.Addr))
	go func() {
		err := d.server.ListenAndServe()
		if err != nil {
			d.logger.Error("Failed to ListenAndServe.", zap.Error(err))
		}
	}()
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), d.server.WriteTimeout)
	defer cancel()

	err := d.server.Shutdown(ctx)
	if err != nil {
		d.logger.Error("Shutdown returned an error", zap.Error(err))
	}
	return err
}

func NewDispatcher(args *InMemoryDispatcherArgs) *InMemoryDispatcher {

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", args.Port),
		Handler:      pkgtracing.HTTPSpanMiddleware(args.Handler),
		ErrorLog:     zap.NewStdLog(args.Logger),
		ReadTimeout:  args.ReadTimeout,
		WriteTimeout: args.WriteTimeout,
	}

	dispatcher := &InMemoryDispatcher{
		handler: args.Handler,
		server:  server,
		logger:  args.Logger,
	}

	return dispatcher
}
