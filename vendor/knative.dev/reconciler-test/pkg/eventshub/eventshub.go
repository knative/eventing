/*
Copyright 2020 The Knative Authors

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

package eventshub

import (
	"context"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	EventGenerators []string `envconfig:"EVENT_GENERATORS" required:"true"`
	EventLogs       []string `envconfig:"EVENT_LOGS" required:"true"`
}

// EventLogFactory creates a new EventLog instance.
type EventLogFactory func(context.Context) (EventLog, error)

// EventGeneratorStarter starts a new event generator. This function is executed in a separate goroutine, so it can block.
type EventGeneratorStarter func(context.Context, *EventLogs) error

// Start starts a new eventshub process, with the provided factories.
// You can create your own eventshub providing event log factories and event generator factories.
func Start(eventLogFactories map[string]EventLogFactory, eventGeneratorFactories map[string]EventGeneratorStarter) {
	ctx := signals.NewContext()
	defer maybeQuitIstioProxy(ctx) // quit at exit
	ctx, _ = injection.EnableInjectionOrDie(ctx, nil)
	ctx = ConfigureLogging(ctx, "eventshub")

	mp, tp, err := ConfigureObservability(ctx, logging.FromContext(ctx), "eventshub")
	if err != nil {
		logging.FromContext(ctx).Fatal("Unable to setup observability", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := mp.Shutdown(ctx); err != nil {
			logging.FromContext(ctx).Warnw("Failed to shutdown metrics provider", zap.Error(err))
		}
		if err := tp.Shutdown(ctx); err != nil {
			logging.FromContext(ctx).Warnw("Failed to shutdown tracer provider", zap.Error(err))
		}
	}()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}
	logging.FromContext(ctx).Infof("Events Hub environment configuration: %+v", env)

	eventLogs := createEventLogs(ctx, eventLogFactories, env.EventLogs)
	err = startEventGenerators(ctx, eventGeneratorFactories, env.EventGenerators, eventLogs)

	if err != nil {
		logging.FromContext(ctx).Fatal("Error during start: ", err)
	}

	logging.FromContext(ctx).Info("Closing the eventshub process")
}

func createEventLogs(ctx context.Context, factories map[string]EventLogFactory, logTypes []string) *EventLogs {
	var eventLogs []EventLog
	for _, logType := range logTypes {
		factory, ok := factories[logType]
		if !ok {
			logging.FromContext(ctx).Fatal("Cannot recognize event log type: ", logType)
		}

		eventLog, err := factory(ctx)
		if err != nil {
			logging.FromContext(ctx).Fatalf("Error while instantiating the event log %s: %s", logType, err)
		}

		eventLogs = append(eventLogs, eventLog)
	}
	return NewEventLogs(eventLogs...)
}

func startEventGenerators(ctx context.Context, factories map[string]EventGeneratorStarter, genTypes []string, eventLogs *EventLogs) error {
	errs, _ := errgroup.WithContext(ctx)
	for _, genType := range genTypes {
		factory, ok := factories[genType]
		if !ok {
			logging.FromContext(ctx).Fatal("Cannot recognize event generator type: ", genType)
		}

		errs.Go(func() error {
			return factory(ctx, eventLogs)
		})
	}
	return errs.Wait()
}
