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
	"errors"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/channel/swappable"
	"knative.dev/eventing/pkg/kncloudevents"
)

type Dispatcher interface {
	UpdateConfig(config *multichannelfanout.Config) error
}

type InMemoryDispatcher struct {
	handler      *swappable.Handler
	ceClient     cloudevents.Client
	writeTimeout time.Duration
	logger       *zap.Logger
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
func (d *InMemoryDispatcher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.ceClient.StartReceiver(ctx, d.handler.ServeHTTP)
	}()

	// Stop either if the receiver stops (sending to errCh) or if the context Done channel is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		break
	}

	// Done channel has been closed, we need to gracefully shutdown d.ceClient. The cancel() method will start its
	// shutdown, if it hasn't finished in a reasonable amount of time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(d.writeTimeout):
		return errors.New("timeout shutting down ceClient")
	}
}

func NewDispatcher(args *InMemoryDispatcherArgs) *InMemoryDispatcher {
	// TODO set read and write timeouts and port?
	ceClient, err := kncloudevents.NewDefaultClient()
	if err != nil {
		args.Logger.Fatal("failed to create cloudevents client", zap.Error(err))
	}

	dispatcher := &InMemoryDispatcher{
		handler:  args.Handler,
		ceClient: ceClient,
		logger:   args.Logger,
	}

	return dispatcher
}
