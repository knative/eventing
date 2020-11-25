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

package main

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/reconciler-test/pkg/test_images"
	"knative.dev/reconciler-test/pkg/test_images/eventshub"
	"knative.dev/reconciler-test/pkg/test_images/eventshub/logger_vent"
	"knative.dev/reconciler-test/pkg/test_images/eventshub/receiver"
	"knative.dev/reconciler-test/pkg/test_images/eventshub/recorder_vent"
	"knative.dev/reconciler-test/pkg/test_images/eventshub/sender"
)

type envConfig struct {
	EventGenerators []string `envconfig:"EVENT_GENERATORS" required:"true"`
	EventLogs       []string `envconfig:"EVENT_LOGS" required:"true"`
}

func main() {
	//nolint // nil ctx is fine here, look at the code of EnableInjectionOrDie
	ctx, _ := injection.EnableInjectionOrDie(nil, nil)
	ctx = test_images.ConfigureLogging(ctx, "eventshub")

	if err := test_images.ConfigureTracing(logging.FromContext(ctx), ""); err != nil {
		logging.FromContext(ctx).Fatal("Unable to setup trace publishing", err)
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}
	logging.FromContext(ctx).Infof("Events Hub environment configuration: %+v", env)

	eventLogs := createEventLogs(ctx, env.EventLogs)
	err := startEventGenerators(ctx, env.EventGenerators, eventLogs)

	if err != nil {
		logging.FromContext(ctx).Fatal("Error during start: ", err)
	}

	logging.FromContext(ctx).Info("Closing the eventshub process")
}

func createEventLogs(ctx context.Context, logTypes []string) *eventshub.EventLogs {
	var l []eventshub.EventLog
	for _, logType := range logTypes {
		switch eventshub.EventLogType(logType) {
		case eventshub.RecorderEventLog:
			l = append(l, recorder_vent.NewFromEnv(ctx))
		case eventshub.LoggerEventLog:
			l = append(l, logger_vent.Logger(logging.FromContext(ctx).Named("event logger").Infof))
		default:
			logging.FromContext(ctx).Fatal("Cannot recognize event log type: ", logType)
		}
	}
	return eventshub.NewEventLogs(l...)
}

func startEventGenerators(ctx context.Context, genTypes []string, eventLogs *eventshub.EventLogs) error {
	errs, _ := errgroup.WithContext(ctx)
	for _, genType := range genTypes {
		switch eventshub.EventGeneratorType(genType) {
		case eventshub.ReceiverEventGenerator:
			errs.Go(func() error {
				return receiver.NewFromEnv(ctx, eventLogs).Start(ctx, test_images.WithTracing)
			})
		case eventshub.SenderEventGenerator:
			errs.Go(func() error {
				return sender.Start(ctx, eventLogs)
			})
		default:
			logging.FromContext(ctx).Fatal("Cannot recognize event generator type: ", genType)
		}
	}
	return errs.Wait()
}
