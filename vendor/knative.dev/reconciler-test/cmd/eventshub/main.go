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

	"knative.dev/pkg/logging"
	"knative.dev/reconciler-test/pkg/eventshub/forwarder"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/logger_vent"
	"knative.dev/reconciler-test/pkg/eventshub/receiver"
	"knative.dev/reconciler-test/pkg/eventshub/recorder_vent"
	"knative.dev/reconciler-test/pkg/eventshub/sender"
)

func main() {
	eventshub.Start(
		map[string]eventshub.EventLogFactory{
			eventshub.RecorderEventLog: func(ctx context.Context) (eventshub.EventLog, error) {
				return recorder_vent.NewFromEnv(ctx), nil
			},
			eventshub.LoggerEventLog: func(ctx context.Context) (eventshub.EventLog, error) {
				return logger_vent.Logger(logging.FromContext(ctx).Named("event logger").Infof), nil
			},
		},
		map[string]eventshub.EventGeneratorStarter{
			eventshub.ReceiverEventGenerator: func(ctx context.Context, logs *eventshub.EventLogs) error {
				return receiver.NewFromEnv(ctx, logs).Start(ctx, eventshub.WithServerTracing)
			},
			eventshub.SenderEventGenerator: func(ctx context.Context, logs *eventshub.EventLogs) error {
				return sender.Start(ctx, logs, eventshub.WithClientTracing)
			},
			eventshub.ForwarderEventGenerator: func(ctx context.Context, logs *eventshub.EventLogs) error {
				return forwarder.NewFromEnv(ctx, logs,
					[]eventshub.HandlerFunc{eventshub.WithServerTracing},
					[]eventshub.ClientOption{eventshub.WithClientTracing}).Start(ctx)
			},
		},
	)
}
